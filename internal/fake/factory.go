/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package fake

import (
	"context"
	"os"

	"k8s.io/utils/ptr"

	internalclient "github.com/gardener/gardenctl-v2/internal/client"
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

// Factory implements util.Factory interface.
type Factory struct {
	// ContextImpl is the root context any command should use.
	ContextImpl context.Context

	// Either set a specific Manager instance, or overwrite the
	// individual providers/caches down below.
	ManagerImpl target.Manager

	// Override these to customize the created manager.
	Config             *config.Config
	ClientProviderImpl internalclient.Provider
	TargetProviderImpl target.TargetProvider
	TargetFlagsImpl    target.TargetFlags

	// Override the clock implementation. Will use a real clock if not set.
	ClockImpl util.Clock

	// GardenHomeDirectory is the home directory for all gardenctl
	// related files. While some files can be explicitly loaded from
	// different locations, persistent cache files will always be placed
	// inside the garden home.
	GardenHomeDirectory string

	// GardenTempDirectory is the base directory for temporary data.
	GardenTempDirectory string
}

var _ util.Factory = &Factory{}

func NewFakeFactory(cfg *config.Config, clock util.Clock, clientProvider internalclient.Provider, targetProvider target.TargetProvider) *Factory {
	if cfg == nil {
		cfg = &config.Config{
			LinkKubeconfig: ptr.To(false),
		}
	}

	if targetProvider == nil {
		targetProvider = NewFakeTargetProvider(nil)
	}

	if clock == nil {
		clock = &util.RealClock{}
	}

	targetFlags := target.NewTargetFlags("", "", "", "", false)

	return &Factory{
		Config:             cfg,
		ClockImpl:          clock,
		ClientProviderImpl: clientProvider,
		TargetProviderImpl: targetProvider,
		TargetFlagsImpl:    targetFlags,
	}
}

func (f *Factory) Manager() (target.Manager, error) {
	if f.ManagerImpl != nil {
		return f.ManagerImpl, nil
	}

	sessionDir := os.TempDir()

	return target.NewManager(f.Config, f.TargetProviderImpl, f.ClientProviderImpl, sessionDir)
}

func (f *Factory) Context() context.Context {
	if f.ContextImpl != nil {
		return f.ContextImpl
	}

	return context.Background()
}

func (f *Factory) GardenHomeDir() string {
	return f.GardenHomeDirectory
}

func (f *Factory) GardenTempDir() string {
	return f.GardenTempDirectory
}

func (f *Factory) Clock() util.Clock {
	return f.ClockImpl
}

func (f *Factory) PublicIPs(_ context.Context) ([]string, error) {
	return []string{"192.0.2.42", "2001:db8::8a2e:370:7334"}, nil
}

func (f *Factory) TargetFlags() target.TargetFlags {
	return f.TargetFlagsImpl
}
