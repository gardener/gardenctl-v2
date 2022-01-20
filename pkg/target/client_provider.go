/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/
package target

import (
	"fmt"

	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:generate mockgen -destination=./mocks/mock_client_provider.go -package=mocks github.com/gardener/gardenctl-v2/pkg/target ClientProvider

// ClientProvider is able to take a kubeconfig either directly or
// from a file and return a controller-runtime client for it.
type ClientProvider interface {
	// FromFile reads a kubeconfig (marshalled as YAML) from a file
	// and returns a Kubernetes client.
	FromFile(kubeconfigFile string) (client.Client, error)
	// FromBytes reads YAML directly and returns a Kubernetes client.
	FromBytes(kubeconfig []byte) (client.Client, error)
	// FromClientConfig returns a Kubernetes client for the given client config.
	FromClientConfig(config clientcmd.ClientConfig) (client.Client, error)
}

type clientProvider struct{}

var _ ClientProvider = &clientProvider{}

// NewClientProvider returns a new ClientProvider.
func NewClientProvider() ClientProvider {
	return &clientProvider{}
}

// FromFile reads a kubeconfig (marshalled as YAML) from a file
// and returns a Kubernetes client.
func (p *clientProvider) FromFile(kubeconfigFile string) (client.Client, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig from %q: %w", kubeconfigFile, err)
	}

	return client.New(config, client.Options{})
}

// FromBytes reads YAML directly and returns a Kubernetes client.
func (p *clientProvider) FromBytes(kubeconfig []byte) (client.Client, error) {
	config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	return client.New(config, client.Options{})
}

// FromClientConfig returns a Kubernetes client for the given client config.
func (p *clientProvider) FromClientConfig(clientConfig clientcmd.ClientConfig) (client.Client, error) {
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create restclient config: %w", err)
	}

	return client.New(config, client.Options{})
}
