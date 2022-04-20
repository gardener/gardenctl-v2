/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package gardenclient

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/Masterminds/semver"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientauthenticationv1beta1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	// ShootProjectSecretSuffixCACluster is a constant for a shoot project secret with suffix 'ca-cluster'.
	ShootProjectSecretSuffixCACluster = "ca-cluster"
	// DataKeyCertificateCA is the key in a secret data holding the CA certificate.
	DataKeyCertificateCA = "ca.crt"
)

// shootKubeconfigRequest is a struct which holds information about a Kubeconfig to be generated.
type shootKubeconfigRequest struct {
	// cluster holds all the cluster on which the kube-apiserver can be reached
	clusters []cluster
	// namespace is the namespace where the shoot resides
	namespace string
	// shootName is the name of the shoot
	shootName string
	// gardenClusterIdentity is the cluster identifier of the garden cluster.
	gardenClusterIdentity string
}

// cluster holds the data to describe and connect to a kubernetes cluster
type cluster struct {
	// name is the name of the shoot advertised address, usually "external", "internal" or "unmanaged"
	name string
	// apiServerHost is the host of the kube-apiserver
	apiServerHost string

	// caCert holds the ca certificate for the cluster
	//+optional
	caCert []byte
}

// ExecPluginConfig contains a reference to the garden and shoot cluster
type ExecPluginConfig struct {
	// ShootRef references the shoot cluster
	ShootRef ShootRef `json:"shootRef"`
	// GardenClusterIdentity is the cluster identifier of the garden cluster.
	// See cluster-identity ConfigMap in kube-system namespace of the garden cluster
	GardenClusterIdentity string `json:"gardenClusterIdentity"`
}

// ShootRef references the shoot cluster by namespace and name
type ShootRef struct {
	// Namespace is the namespace of the shoot cluster
	Namespace string `json:"namespace"`
	// Name is the name of the shoot cluster
	Name string `json:"name"`
}

func (e *ExecPluginConfig) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (e *ExecPluginConfig) DeepCopyObject() runtime.Object {
	return &ExecPluginConfig{
		ShootRef: ShootRef{
			Namespace: e.ShootRef.Namespace,
			Name:      e.ShootRef.Name,
		},
		GardenClusterIdentity: e.GardenClusterIdentity,
	}
}

// validate validates the kubeconfig request by ensuring that all required fields are set
func (k *shootKubeconfigRequest) validate() error {
	if len(k.clusters) == 0 {
		return errors.New("missing clusters")
	}

	for n, cluster := range k.clusters {
		if cluster.name == "" {
			return fmt.Errorf("no name defined for cluster[%d]", n)
		}

		if cluster.apiServerHost == "" {
			return fmt.Errorf("no api server host defined for cluster[%d]", n)
		}
	}

	if k.namespace == "" {
		return errors.New("no namespace defined for kubeconfig request")
	}

	if k.shootName == "" {
		return errors.New("no shoot name defined for kubeconfig request")
	}

	if k.gardenClusterIdentity == "" {
		return errors.New("no garden cluster identity defined for kubeconfig request")
	}

	return nil
}

// generate generates a Kubernetes kubeconfig for communicating with the kube-apiserver
// by exec'ing the gardenlogin plugin, which fetches a client certificate.
// If legacy is false, the shoot reference and garden cluster identity is passed via the cluster extensions,
// which is supported starting with kubectl version v1.20.0.
// If legacy is true, the shoot reference and garden cluster identity are passed as command line flags to the plugin
func (k *shootKubeconfigRequest) generate(legacy bool) (*clientcmdapi.Config, error) {
	var extension *ExecPluginConfig

	args := []string{
		"gardenlogin",
		"get-client-certificate",
	}

	if legacy {
		args = append(
			args,
			fmt.Sprintf("--name=%s", k.shootName),
			fmt.Sprintf("--namespace=%s", k.namespace),
			fmt.Sprintf("--garden-cluster-identity=%s", k.gardenClusterIdentity),
		)
	} else {
		extension = &ExecPluginConfig{
			ShootRef: ShootRef{
				Namespace: k.namespace,
				Name:      k.shootName,
			},
			GardenClusterIdentity: k.gardenClusterIdentity,
		}
	}

	config := clientcmdapi.NewConfig()

	authInfo := clientcmdapi.NewAuthInfo()
	authInfo.Exec = &clientcmdapi.ExecConfig{
		Command:            "kubectl",
		Args:               args,
		Env:                nil,
		APIVersion:         clientauthenticationv1beta1.SchemeGroupVersion.String(),
		InstallHint:        "",
		ProvideClusterInfo: true,
	}
	authName := fmt.Sprintf("%s--%s", k.namespace, k.shootName) // TODO instead of namespace, use project? But this would require an additional call
	config.AuthInfos[authName] = authInfo

	for i, c := range k.clusters {
		name := fmt.Sprintf("%s-%s", authName, c.name)
		if i == 0 {
			config.CurrentContext = name
		}

		cluster := clientcmdapi.NewCluster()
		cluster.CertificateAuthorityData = c.caCert
		cluster.Server = fmt.Sprintf("https://%s", c.apiServerHost)

		if !legacy {
			cluster.Extensions["client.authentication.k8s.io/exec"] = extension
		}

		config.Clusters[name] = cluster

		context := clientcmdapi.NewContext()
		context.Cluster = name
		context.AuthInfo = authName
		context.Namespace = "default" // TODO leave hardcoded? Or omit?
		config.Contexts[name] = context
	}

	return config, nil
}

func (g *clientImpl) GetShootClientConfig(ctx context.Context, namespace, name string) (clientcmd.ClientConfig, error) {
	if len(g.name) == 0 {
		return nil, errors.New("garden name must not be empty")
	}

	// fetch Shoot
	shoot := &gardencorev1beta1.Shoot{}
	key := types.NamespacedName{Namespace: namespace, Name: name}

	if err := g.c.Get(ctx, key, shoot); err != nil {
		return nil, err
	}

	if len(shoot.Status.AdvertisedAddresses) == 0 {
		return nil, errors.New("no advertised addresses listed in the Shoot status for the Shoot Kube API server")
	}

	// fetch cluster ca
	caClusterSecret := corev1.Secret{}
	caClusterSecretName := fmt.Sprintf("%s.%s", name, ShootProjectSecretSuffixCACluster)

	if err := g.c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: caClusterSecretName}, &caClusterSecret); err != nil {
		return nil, err
	}

	caCert, ok := caClusterSecret.Data[DataKeyCertificateCA]
	if !ok || len(caCert) == 0 {
		return nil, fmt.Errorf("%s of secret %s is empty", DataKeyCertificateCA, caClusterSecretName)
	}

	kubeconfigRequest := shootKubeconfigRequest{
		namespace:             shoot.Namespace,
		shootName:             shoot.Name,
		gardenClusterIdentity: g.name,
	}

	for _, address := range shoot.Status.AdvertisedAddresses {
		u, err := url.Parse(address.URL)
		if err != nil {
			return nil, fmt.Errorf("could not parse shoot server url: %w", err)
		}

		kubeconfigRequest.clusters = append(kubeconfigRequest.clusters, cluster{
			name:          address.Name,
			apiServerHost: u.Host,
			caCert:        caCert,
		})
	}

	if err := kubeconfigRequest.validate(); err != nil {
		return nil, fmt.Errorf("validation failed for kubeconfig request: %w", err)
	}

	// parse kubernetes version to determine if a legacy kubeconfig should be created.
	constraint, err := semver.NewConstraint("< v1.20.0")
	if err != nil {
		return nil, fmt.Errorf("failed to parse constraint: %w", err)
	}

	version, err := semver.NewVersion(shoot.Spec.Kubernetes.Version)
	if err != nil {
		return nil, fmt.Errorf("could not parse kubernetes version %s of shoot cluster: %w", shoot.Spec.Kubernetes.Version, err)
	}

	legacy := constraint.Check(version)

	config, err := kubeconfigRequest.generate(legacy)
	if err != nil {
		return nil, fmt.Errorf("generation failed for kubeconfig request: %w", err)
	}

	return clientcmd.NewDefaultClientConfig(*config, nil), nil
}
