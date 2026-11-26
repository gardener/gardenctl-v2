/*
SPDX-FileCopyrightText: Contributors to the Gardener project

SPDX-License-Identifier: Apache-2.0
*/

package fake

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gardener/gardenctl-v2/internal/client/garden"
	"github.com/gardener/gardenctl-v2/pkg/config"
)

// NewConfigData generates a Kubernetes client configuration as a byte slice.
func NewConfigData(name string) ([]byte, error) {
	return clientcmd.Write(*NewTokenConfig(name))
}

// NewSignedSeedKubeconfigSecret creates a seed login Secret and its matching test trust configuration.
func NewSignedSeedKubeconfigSecret(seedName string) (*corev1.Secret, config.SeedKubeconfigPublicKeys, error) {
	kubeconfig, err := NewConfigData(seedName)
	if err != nil {
		return nil, nil, err
	}

	manifest, err := json.Marshal(garden.SeedKubeconfigSignatureManifest{
		APIVersion:     garden.SeedKubeconfigSignatureAPIVersion,
		Kind:           garden.SeedKubeconfigSignatureKind,
		SeedName:       seedName,
		KubeconfigHash: fmt.Sprintf("sha256:%x", sha256.Sum256(kubeconfig)),
	})
	if err != nil {
		return nil, nil, err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      seedName + ".login",
			Namespace: "garden",
			UID:       "00000000-0000-0000-0000-000000000000",
		},
		Data: map[string][]byte{
			"kubeconfig":                         kubeconfig,
			garden.SeedKubeconfigManifestDataKey: manifest,
		},
	}

	keys, err := SignSeedKubeconfigManifest(secret)
	if err != nil {
		return nil, nil, err
	}

	return secret, keys, nil
}

// SignSeedKubeconfigManifest signs the manifest bytes already present in the Secret.
// It generates a fresh test key and returns the matching public key configuration.
func SignSeedKubeconfigManifest(secret *corev1.Secret) (config.SeedKubeconfigPublicKeys, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	manifestHash := sha256.Sum256(secret.Data[garden.SeedKubeconfigManifestDataKey])

	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, manifestHash[:])
	if err != nil {
		return nil, err
	}

	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, err
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyDER})
	secret.Data[garden.SeedKubeconfigSignatureDataKey] = signature

	return config.SeedKubeconfigPublicKeys{string(publicKeyPEM)}, nil
}

// NewTokenConfig generates a new Kubernetes client configuration
// with token authentication info.
func NewTokenConfig(name string) *clientcmdapi.Config {
	config := clientcmdapi.NewConfig()
	config.Clusters["cluster"] = &clientcmdapi.Cluster{
		Server: "https://kubernetes:6443/",
	}
	config.AuthInfos["user"] = &clientcmdapi.AuthInfo{
		Token: "token",
	}
	config.Contexts[name] = &clientcmdapi.Context{
		Namespace: "default",
		AuthInfo:  "user",
		Cluster:   "cluster",
	}
	config.CurrentContext = name

	return config
}

func NewCertConfig(name string, clientCert []byte) *clientcmdapi.Config {
	config := clientcmdapi.NewConfig()
	config.Clusters["cluster"] = &clientcmdapi.Cluster{
		Server: "https://kubernetes:6443/",
	}
	config.AuthInfos["user"] = &clientcmdapi.AuthInfo{
		ClientCertificateData: clientCert,
	}
	config.Contexts[name] = &clientcmdapi.Context{
		Namespace: "default",
		AuthInfo:  "user",
		Cluster:   "cluster",
	}
	config.CurrentContext = name

	return config
}
