/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/
package fake

import (
	"fmt"

	"github.com/gardener/gardenctl-v2/pkg/target"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ClientProvider struct {
	fakeClients map[string]client.Client
}

var _ target.ClientProvider = &ClientProvider{}

// NewFakeClientProvider returns a new ClientProvider that returns a static
// client for a given kubeconfig / kubeconfig file.
func NewFakeClientProvider() *ClientProvider {
	return &ClientProvider{
		fakeClients: map[string]client.Client{},
	}
}

// WithClient adds an additional client to the provider, which it will
// return whenever a consumer requests a client with the same key.
func (p *ClientProvider) WithClient(key string, c client.Client) *ClientProvider {
	p.fakeClients[key] = c
	return p
}

func (p *ClientProvider) tryGetClient(key string) (client.Client, error) {
	if c, ok := p.fakeClients[key]; ok {
		return c, nil
	}

	return nil, fmt.Errorf("no fake client configured for %q", key)
}

func (p *ClientProvider) FromFile(kubeconfigFile string, context string) (client.Client, error) {
	return p.tryGetClient(kubeconfigFile)
}

func (p *ClientProvider) FromBytes(kubeconfig []byte) (client.Client, error) {
	return p.tryGetClient(string(kubeconfig))
}
