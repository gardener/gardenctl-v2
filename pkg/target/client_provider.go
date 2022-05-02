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
	// FromClientConfig returns a Kubernetes client for the given client config.
	FromClientConfig(config clientcmd.ClientConfig) (client.Client, error)
}

type clientProvider struct{}

var _ ClientProvider = &clientProvider{}

// NewClientProvider returns a new ClientProvider.
func NewClientProvider() ClientProvider {
	return &clientProvider{}
}

// FromClientConfig returns a Kubernetes client for the given client config.
func (p *clientProvider) FromClientConfig(clientConfig clientcmd.ClientConfig) (client.Client, error) {
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create restclient config: %w", err)
	}

	return client.New(config, client.Options{})
}
