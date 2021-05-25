/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/
package target

import (
	"errors"
)

/*
	Targets are either

	garden -> seed -> shoot
	garden -> project -> shoot
*/

// Target represents the Kubernetes cluster/namespace which should
// be the target for user operations in gardenctl. It works similar
// to the context defined in a kubeconfig.
type Target interface {
	// GardenName returns the currently targeted garden cluster name.
	GardenName() string
	// ProjectName returns the currently targeted project name.
	ProjectName() string
	// SeedName returns the currently targeted seed cluster name.
	SeedName() string
	// ShootName returns the currently targeted shoot cluster name.
	ShootName() string
}

type targetImpl struct {
	Garden  string `yaml:"garden,omitempty"`
	Project string `yaml:"project,omitempty"`
	Seed    string `yaml:"seed,omitempty"`
	Shoot   string `yaml:"shoot,omitempty"`
	// TODO: Namespace
}

var _ Target = &targetImpl{}

// NewTarget should mostly be used in tests. Regular program code should always
// use the Manager to read/save the current target. This function does not
// perform any validation, so the returned target can be invalid.
func NewTarget(gardenName, projectName, seedName, shootName string) Target {
	return &targetImpl{
		Garden:  gardenName,
		Project: projectName,
		Seed:    seedName,
		Shoot:   shootName,
	}
}

// Validate checks that the target is not malformed and all required
// steps are configured correctly.
func (t *targetImpl) Validate() error {
	if len(t.Project) > 0 && len(t.Seed) > 0 {
		return errors.New("seed and project must not be configured at the same time")
	}

	return nil
}

// GardenName returns the currently targeted garden cluster name.
func (t *targetImpl) GardenName() string {
	return t.Garden
}

// ProjectName returns the currently targeted project name.
func (t *targetImpl) ProjectName() string {
	return t.Project
}

// SeedName returns the currently targeted seed cluster name.
func (t *targetImpl) SeedName() string {
	return t.Seed
}

// ShootName returns the currently targeted shoot cluster name.
func (t *targetImpl) ShootName() string {
	return t.Shoot
}
