/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"strconv"

	"github.com/spf13/pflag"
)

// TargetFlags represents the target cobra flags.
type TargetFlags interface {
	// GardenName returns the value that is tied to the corresponding cobra flag.
	GardenName() string
	// ProjectName returns the value that is tied to the corresponding cobra flag.
	ProjectName() string
	// SeedName returns the value that is tied to the corresponding cobra flag.
	SeedName() string
	// ShootName returns the value that is tied to the corresponding cobra flag.
	ShootName() string
	// ControlPlane returns the value and presence state that are tied to the corresponding cobra flag.
	ControlPlane() BoolFlag

	// AddFlags binds all target configuration flags to a given flagset
	AddFlags(flags *pflag.FlagSet)
	// AddGardenFlag adds the garden flag to the provided flag set
	AddGardenFlag(flags *pflag.FlagSet)
	// AddProjectFlag adds the project flag to the provided flag set
	AddProjectFlag(flags *pflag.FlagSet)
	// AddSeedFlag adds the seed flag to the provided flag set
	AddSeedFlag(flags *pflag.FlagSet)
	// AddShootFlag adds the shoot flag to the provided flag set
	AddShootFlag(flags *pflag.FlagSet)
	// AddControlPlaneFlag adds the control-plane flag to the provided flag set
	AddControlPlaneFlag(flags *pflag.FlagSet)

	// ToTarget converts the flags to a target
	ToTarget() Target
	// IsTargetValid returns true if the set of given CLI flags is enough
	// to create a meaningful target. For example, if only the SeedName is
	// given, false is returned because for targeting a seed, the GardenName
	// must also be given. If ShootName and GardenName are set, false is
	// returned because either project or seed have to be given as well.
	IsTargetValid() bool

	// IsEmpty returns true if no flags were given by the user
	IsEmpty() bool
}

func NewTargetFlags(garden, project, seed, shoot string, controlPlane bool) TargetFlags {
	controlPlaneFlag := NewBoolFlag(false)
	if controlPlane {
		controlPlaneFlag = newProvidedBoolFlag(true)
	}

	return &targetFlagsImpl{
		gardenName:   garden,
		projectName:  project,
		seedName:     seed,
		shootName:    shoot,
		controlPlane: controlPlaneFlag,
	}
}

type targetFlagsImpl struct {
	gardenName   string
	projectName  string
	seedName     string
	shootName    string
	controlPlane BoolFlag
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

func (tf *targetFlagsImpl) ControlPlane() BoolFlag {
	return tf.controlPlane
}

func (tf *targetFlagsImpl) AddFlags(flags *pflag.FlagSet) {
	tf.AddGardenFlag(flags)
	tf.AddProjectFlag(flags)
	tf.AddSeedFlag(flags)
	tf.AddShootFlag(flags)
	tf.AddControlPlaneFlag(flags)
}

func (tf *targetFlagsImpl) AddGardenFlag(flags *pflag.FlagSet) {
	flags.StringVar(&tf.gardenName, "garden", "", "target the given garden cluster")
}

func (tf *targetFlagsImpl) AddProjectFlag(flags *pflag.FlagSet) {
	flags.StringVar(&tf.projectName, "project", "", "target the given project")
}

func (tf *targetFlagsImpl) AddSeedFlag(flags *pflag.FlagSet) {
	flags.StringVar(&tf.seedName, "seed", "", "target the given seed cluster")
}

func (tf *targetFlagsImpl) AddShootFlag(flags *pflag.FlagSet) {
	flags.StringVar(&tf.shootName, "shoot", "", "target the given shoot cluster")
}

func (tf *targetFlagsImpl) AddControlPlaneFlag(flags *pflag.FlagSet) {
	flags.Var(&tf.controlPlane, "control-plane", "target control plane of shoot, use together with shoot argument")
	flags.Lookup("control-plane").NoOptDefVal = "true"
}

func (tf *targetFlagsImpl) ToTarget() Target {
	return NewTarget(tf.gardenName, tf.projectName, tf.seedName, tf.shootName).WithControlPlane(tf.ControlPlane().Value())
}

func (tf *targetFlagsImpl) IsEmpty() bool {
	return tf.gardenName == "" && tf.projectName == "" && tf.seedName == "" && tf.shootName == "" && !tf.ControlPlane().Provided()
}

func (tf *targetFlagsImpl) IsTargetValid() bool {
	// garden name is always required for a complete set of flags
	if tf.gardenName == "" {
		return false
	}

	return tf.ToTarget().Validate() == nil
}

// BoolFlag is a boolean flag compatible with pflag that keeps track of
// whether it had a value supplied or not.
type BoolFlag struct {
	// If Set has been invoked this value is true.
	provided bool
	// The exact value provided on the flag.
	value bool
}

func NewBoolFlag(defaultVal bool) BoolFlag {
	return BoolFlag{value: defaultVal}
}

func newProvidedBoolFlag(value bool) BoolFlag {
	return BoolFlag{provided: true, value: value}
}

func (f BoolFlag) String() string {
	return strconv.FormatBool(f.value)
}

func (f BoolFlag) Value() bool {
	return f.value
}

func (f *BoolFlag) Set(value string) error {
	boolVal, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}

	f.value = boolVal
	f.provided = true

	return nil
}

func (f BoolFlag) Provided() bool {
	return f.provided
}

func (f *BoolFlag) Type() string {
	return "bool"
}

func (f *BoolFlag) IsBoolFlag() bool {
	return true
}
