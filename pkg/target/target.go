/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/
package target

import (
	"errors"
	"fmt"
	"strings"
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
	// WithGardenName returns a copy of the target with the garden name updated.
	// The returned target can be invalid.
	WithGardenName(name string) Target
	// WithProjectName returns a copy of the target with the project name updated.
	// The returned target can be invalid.
	WithProjectName(name string) Target
	// WithSeedName returns a copy of the target with the seed name updated.
	// The returned target can be invalid.
	WithSeedName(name string) Target
	// WithShootName returns a copy of the target with the shoot name updated.
	// The returned target can be invalid.
	WithShootName(name string) Target
	// Validate checks for semantical correctness of the target, without
	// actually connecting to the targeted clusters.
	Validate() error
}

type targetImpl struct {
	Garden  string `yaml:"garden,omitempty"`
	Project string `yaml:"project,omitempty"`
	Seed    string `yaml:"seed,omitempty"`
	Shoot   string `yaml:"shoot,omitempty"`
}

var _ Target = &targetImpl{}

// NewTarget returns a new target. This function does not perform any validation,
// so the returned target can be invalid.
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

// WithGardenName returns a copy of the target with the garden name updated.
// The returned target can be invalid.
func (t *targetImpl) WithGardenName(name string) Target {
	return NewTarget(name, t.Project, t.Seed, t.Shoot)
}

// WithProjectName returns a copy of the target with the project name updated.
// The returned target can be invalid.
func (t *targetImpl) WithProjectName(name string) Target {
	return NewTarget(t.Garden, name, t.Seed, t.Shoot)
}

// WithSeedName returns a copy of the target with the seed name updated.
// The returned target can be invalid.
func (t *targetImpl) WithSeedName(name string) Target {
	return NewTarget(t.Garden, t.Project, name, t.Shoot)
}

// WithShootName returns a copy of the target with the shoot name updated.
// The returned target can be invalid.
func (t *targetImpl) WithShootName(name string) Target {
	return NewTarget(t.Garden, t.Project, t.Seed, name)
}

// String returns a readable representation of the target.
func (t *targetImpl) String() string {
	steps := []string{}

	if t.Garden != "" {
		steps = append(steps, fmt.Sprintf("garden:%q", t.Garden))
	}

	if t.Project != "" {
		steps = append(steps, fmt.Sprintf("project:%q", t.Project))
	}

	if t.Seed != "" {
		steps = append(steps, fmt.Sprintf("seed:%q", t.Seed))
	}

	if t.Shoot != "" {
		steps = append(steps, fmt.Sprintf("shoot:%q", t.Shoot))
	}

	if len(steps) == 0 {
		return "<empty>"
	}

	return strings.Join(steps, ", ")
}
