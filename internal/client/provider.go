/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package client

import (
	"fmt"

	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:generate mockgen -destination=./mocks/mock_client_provider.go -package=mocks github.com/gardener/gardenctl-v2/pkg/target Provider

// Provider is able to take a kubeconfig either directly or
// from a file and return a controller-runtime client for it.
type Provider interface {
	// FromClientConfig returns a Kubernetes client for the given client config.
	FromClientConfig(config clientcmd.ClientConfig) (client.Client, error)
}

type provider struct{}

var _ Provider = &provider{}

// NewProvider returns a new Provider.
func NewProvider() Provider {
	return &provider{}
}

// FromClientConfig returns a Kubernetes client for the given client config.
func (p *provider) FromClientConfig(clientConfig clientcmd.ClientConfig) (client.Client, error) {
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create restclient config: %w", err)
	}

	return client.New(config, client.Options{})
}
