/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package fake

import (
	"context"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

// Factory implements util.Factory interface
type Factory struct {
	// Either set a specific Manager instance, or overwrite the
	// individual providers/caches down below.
	ManagerImpl target.Manager

	// Override these to customize the created manager.
	Config              *config.Config
	ClientProviderImpl  target.ClientProvider
	KubeconfigCacheImpl target.KubeconfigCache
	TargetProviderImpl  target.TargetProvider

	// Override the clock implementation. Will use a real clock if not set.
	ClockImpl util.Clock

	// GardenHomeDirectory is the home directory for all gardenctl
	// related files. While some files can be explicitly loaded from
	// different locations, cache files will always be placed inside
	// the garden home.
	GardenHomeDirectory string
}

var _ util.Factory = &Factory{}

func NewFakeManagerFactory(manager target.Manager) util.Factory {
	return &Factory{
		ManagerImpl: manager,
	}
}

func NewFakeFactory(cfg *config.Config, clock util.Clock, clientProvider target.ClientProvider, kubeconfigCache target.KubeconfigCache, targetProvider target.TargetProvider) util.Factory {
	if cfg == nil {
		cfg = &config.Config{}
	}

	if clientProvider == nil {
		clientProvider = NewFakeClientProvider(nil)
	}

	if kubeconfigCache == nil {
		kubeconfigCache = NewFakeKubeconfigCache()
	}

	if targetProvider == nil {
		targetProvider = NewFakeTargetProvider(nil)
	}

	if clock == nil {
		clock = &util.RealClock{}
	}

	return &Factory{
		Config:              cfg,
		ClockImpl:           clock,
		ClientProviderImpl:  clientProvider,
		KubeconfigCacheImpl: kubeconfigCache,
		TargetProviderImpl:  targetProvider,
	}
}

func (f *Factory) Manager() (target.Manager, error) {
	if f.ManagerImpl != nil {
		return f.ManagerImpl, nil
	}

	return target.NewManager(f.Config, f.TargetProviderImpl, f.ClientProviderImpl, f.KubeconfigCacheImpl)
}

func (f *Factory) GardenHomeDir() string {
	return f.GardenHomeDirectory
}

func (f *Factory) Clock() util.Clock {
	return f.ClockImpl
}

func (f *Factory) PublicIP(ctx context.Context) (string, error) {
	return "192.0.2.42", nil
}