/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package fake

import (
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

// Factory implements util.Factory interface
type Factory struct {
	// Either set a specific Manager instance, or overwrite the
	// individual providers/caches down below.
	ManagerImpl target.Manager

	// Override these to customize the created manager.
	Config              *target.Config
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

	// ConfigFile is the location of the gardenctlv2 configuration file.
	// This can be overriden via a CLI flag and defaults to ~/.garden/gardenctlv2.yaml
	// if empty.
	ConfigFile string

	// TargetFile is the filename where the currently active target is located.
	TargetFile string
}

func NewFakeManagerFactory(manager target.Manager) util.Factory {
	return &Factory{
		ManagerImpl: manager,
	}
}

func NewFakeFactory(config *target.Config, clientProvider target.ClientProvider, kubeconfigCache target.KubeconfigCache, targetProvider target.TargetProvider) util.Factory {
	if config == nil {
		config = &target.Config{}
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

	return &Factory{
		Config:              config,
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

func (f *Factory) HomeDir() string {
	return f.GardenHomeDirectory
}

func (f *Factory) Clock() util.Clock {
	if f.ClockImpl != nil {
		return f.ClockImpl
	}

	return &util.RealClock{}
}
