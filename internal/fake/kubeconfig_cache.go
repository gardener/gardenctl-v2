/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package fake

import (
	"fmt"

	"github.com/gardener/gardenctl-v2/pkg/target"
)

type fakeKubeconfigCache struct {
	kubeconfigs map[string][]byte
}

var _ target.KubeconfigCache = &fakeKubeconfigCache{}

func NewFakeKubeconfigCache() target.KubeconfigCache {
	return &fakeKubeconfigCache{
		kubeconfigs: map[string][]byte{},
	}
}

func (c *fakeKubeconfigCache) key(t target.Target) string {
	return fmt.Sprintf("%s;%s;%s;%s", t.GardenName(), t.ProjectName(), t.SeedName(), t.ShootName())
}

func (c *fakeKubeconfigCache) Read(t target.Target) ([]byte, error) {
	kubeconfig, ok := c.kubeconfigs[c.key(t)]
	if !ok {
		return nil, fmt.Errorf("could not find kubeconfig for target %v", c.key(t))
	}

	return kubeconfig, nil
}

func (c *fakeKubeconfigCache) Write(t target.Target, kubeconfig []byte) error {
	c.kubeconfigs[c.key(t)] = kubeconfig

	return nil
}
