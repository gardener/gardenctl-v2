/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package util

import (
	"fmt"
	"path/filepath"

	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

// Factory provides abstractions that allow the command to be extended across multiple types of resources and different API sets.
type Factory interface {
	// Clock returns a clock that provides access to the current time.
	Clock() Clock
	// HomeDir returns the home directory for the executing user.
	HomeDir() string
	// Manager returns the target manager used to read and change the currently targeted system.
	Manager() (target.Manager, error)
}

// FactoryImpl implements util.Factory interface
type FactoryImpl struct {
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

var _ Factory = &FactoryImpl{}

func (f *FactoryImpl) Manager() (target.Manager, error) {
	cfg, err := config.LoadFromFile(f.ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	targetProvider := target.NewFilesystemTargetProvider(f.TargetFile)
	kubeconfigCache := target.NewFilesystemKubeconfigCache(filepath.Join(f.GardenHomeDirectory, "cache", "kubeconfigs"))
	clientProvider := target.NewClientProvider()

	return target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
}

func (f *FactoryImpl) HomeDir() string {
	return f.GardenHomeDirectory
}

func (f *FactoryImpl) Clock() Clock {
	return &RealClock{}
}
