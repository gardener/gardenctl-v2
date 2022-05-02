/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"errors"
	"fmt"

	"github.com/spf13/pflag"
)

// TargetFlags represents the target cobra flags.
//nolint
type TargetFlags interface {
	// GardenName returns the value that is tied to the corresponding cobra flag.
	GardenName() string
	// ProjectName returns the value that is tied to the corresponding cobra flag.
	ProjectName() string
	// SeedName returns the value that is tied to the corresponding cobra flag.
	SeedName() string
	// ShootName returns the value that is tied to the corresponding cobra flag.
	ShootName() string
	// ControlPlane returns the value that is tied to the corresponding cobra flag.
	ControlPlane() bool
	// AddFlags binds target configuration flags to a given flagset
	AddFlags(flags *pflag.FlagSet)
	// ToTarget converts the flags to a target
	ToTarget() Target
	// IsTargetValid returns true if the set of given CLI flags is enough
	// to create a meaningful target. For example, if only the SeedName is
	// given, false is returned because for targeting a seed, the GardenName
	// must also be given. If ShootName and GardenName are set, false is
	// returned because either project or seed have to be given as well.
	IsTargetValid() bool
	// OverrideTarget overrides the given target with the values of the target flags
	OverrideTarget(current Target) (Target, error)
}

func NewTargetFlags(garden, project, seed, shoot string, controlPlane bool) TargetFlags {
	return &targetFlagsImpl{
		gardenName:   garden,
		projectName:  project,
		seedName:     seed,
		shootName:    shoot,
		controlPlane: controlPlane,
	}
}

type targetFlagsImpl struct {
	gardenName   string
	projectName  string
	seedName     string
	shootName    string
	controlPlane bool
}

func (tf *targetFlagsImpl) GardenName() string {
	return tf.gardenName
}

func (tf *targetFlagsImpl) ProjectName() string {
	return tf.projectName
}

func (tf *targetFlagsImpl) SeedName() string {
	return tf.seedName
}

func (tf *targetFlagsImpl) ShootName() string {
	return tf.shootName
}

func (tf *targetFlagsImpl) ControlPlane() bool {
	return tf.controlPlane
}

func (tf *targetFlagsImpl) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&tf.gardenName, "garden", "", "target the given garden cluster")
	flags.StringVar(&tf.projectName, "project", "", "target the given project")
	flags.StringVar(&tf.seedName, "seed", "", "target the given seed cluster")
	flags.StringVar(&tf.shootName, "shoot", "", "target the given shoot cluster")
	flags.BoolVar(&tf.controlPlane, "control-plane", tf.controlPlane, "target control plane of shoot, use together with shoot argument")
}

func (tf *targetFlagsImpl) ToTarget() Target {
	return NewTarget(tf.gardenName, tf.projectName, tf.seedName, tf.shootName)
}

func (tf *targetFlagsImpl) isEmpty() bool {
	return tf.gardenName == "" && tf.projectName == "" && tf.seedName == "" && tf.shootName == "" && !tf.controlPlane
}

func (tf *targetFlagsImpl) OverrideTarget(current Target) (Target, error) {
	// user gave _some_ flags; we use those to override the current target
	// (e.g. to quickly change a shoot while keeping garden/project names)
	if !tf.isEmpty() {
		// note that "deeper" levels of targets are reset, as to allow the
		// user to "move up", e.g. when they have targeted a shoot, just
		// specifying "--garden mygarden" should target the garden, not the same
		// shoot on the garden mygarden.
		if tf.gardenName != "" {
			current = current.WithGardenName(tf.gardenName).WithProjectName("").WithSeedName("").WithShootName("")
		}

		if tf.projectName != "" && tf.seedName != "" {
			return nil, errors.New("cannot specify --project and --seed at the same time")
		}

		if tf.projectName != "" {
			current = current.WithProjectName(tf.projectName).WithSeedName("").WithShootName("")
		}

		if tf.seedName != "" {
			current = current.WithSeedName(tf.seedName).WithProjectName("").WithShootName("")
		}

		if tf.shootName != "" {
			current = current.WithShootName(tf.shootName)
		}

		if tf.controlPlane {
			current = current.WithControlPlane(tf.controlPlane)
		}

		if err := current.Validate(); err != nil {
			return nil, fmt.Errorf("invalid target flags: %w", err)
		}
	}

	return current, nil
}

func (tf *targetFlagsImpl) IsTargetValid() bool {
	// garden name is always required for a complete set of flags
	if tf.gardenName == "" {
		return false
	}

	return tf.ToTarget().Validate() == nil
}
