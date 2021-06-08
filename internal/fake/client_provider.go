/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/
package fake

import (
	"errors"

	"github.com/gardener/gardenctl-v2/pkg/target"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type fakeClientProvider struct {
	fakeClient client.Client
}

var _ target.ClientProvider = &fakeClientProvider{}

// NewFakeClientProvider returns a new ClientProvider that returns the same
// client for all FromFile/FromBytes calls.
func NewFakeClientProvider(fakeClient client.Client) target.ClientProvider {
	return &fakeClientProvider{
		fakeClient: fakeClient,
	}
}

func (p *fakeClientProvider) FromFile(kubeconfigFile string) (client.Client, error) {
	if p.fakeClient == nil {
		return nil, errors.New("no fake client configured")
	}

	return p.fakeClient, nil
}

func (p *fakeClientProvider) FromBytes(kubeconfig []byte) (client.Client, error) {
	if p.fakeClient == nil {
		return nil, errors.New("no fake client configured")
	}

	return p.fakeClient, nil
}
