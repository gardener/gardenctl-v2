/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package root

import (
	"errors"
	"fmt"

	"github.com/gardener/gardenctl-v2/pkg/target"
)

// DynamicTargetProvider is a wrapper that combines the basic
// file-based TargetProvider with CLI flags, to allow the user
// to change the target for individual gardenctl commands
// on-the-fly without changing the file on disk every time.
//
// If no CLI flags are given, this functions identical to the
// regular TargetProvider from NewFilesystemTargetProvider().
//
// Otherwise, the flags are used to augment the existing target.
// In this mode, changing the target via Write() is not allowed,
// so that commands do not accidentally overwrite the target on
// disk.
type DynamicTargetProvider struct {
	// TargetFile is the file where the target is read from / written to
	// when no CLI flags override the targeting.
	TargetFile string

	GardenNameFlag  string
	ProjectNameFlag string
	SeedNameFlag    string
	ShootNameFlag   string
}

// hasCLIFlags returns true if _any_ of the *Flag properties are not empty.
func (p *DynamicTargetProvider) hasCLIFlags() bool {
	return p.GardenNameFlag != "" || p.ProjectNameFlag != "" || p.SeedNameFlag != "" || p.ShootNameFlag != ""
}

// hasCompleteCLIFlags returns true if the set of given CLI flags is enough
// to create a meaningful target. For example, if only the SeedNameFlag is
// given, false is returned because for targeting a seed, the GardenNameFlag
// must also be given. If ShootNameFlag and GardenNameFlag are set, false is
// returned because either project or seed have to be given as well.
func (p *DynamicTargetProvider) hasCompleteCLIFlags() bool {
	// garden name is always required for a complete set of flags
	if p.GardenNameFlag == "" {
		return false
	}

	return target.NewTarget(p.GardenNameFlag, p.ProjectNameFlag, p.SeedNameFlag, p.ShootNameFlag).Validate() == nil
}

// Read returns the current target from the TargetFile if no CLI
// flags were given, and tries to construct a meaningful target
// otherwise.
func (p *DynamicTargetProvider) Read() (target.Target, error) {
	// user gave everything we needed
	if p.hasCompleteCLIFlags() {
		return target.NewTarget(p.GardenNameFlag, p.ProjectNameFlag, p.SeedNameFlag, p.ShootNameFlag), nil
	}

	// user didn't specify anything at all or _some_ flags; in both
	// cases we need to read the current target from disk
	current, err := target.NewFilesystemTargetProvider(p.TargetFile).Read()
	if err != nil {
		return nil, err
	}

	// user gave _some_ flags; we use those to override the current target
	// (e.g. to quickly change a shoot while keeping garden/project names)
	if p.hasCLIFlags() {
		// note that "deeper" levels of targets are reset, as to allow the
		// user to "move up", e.g. when they have targeted a shoot, just
		// specifying "--garden mygarden" should target the garden, not the same
		// shoot on the garden mygarden.
		if p.GardenNameFlag != "" {
			current = current.WithGardenName(p.GardenNameFlag).WithProjectName("").WithSeedName("").WithShootName("")
		}

		if p.ProjectNameFlag != "" && p.SeedNameFlag != "" {
			return nil, errors.New("cannot specify --project and --seed at the same time")
		}

		if p.ProjectNameFlag != "" {
			current = current.WithProjectName(p.ProjectNameFlag).WithSeedName("").WithShootName("")
		}

		if p.SeedNameFlag != "" {
			current = current.WithSeedName(p.SeedNameFlag).WithProjectName("").WithShootName("")
		}

		if p.ShootNameFlag != "" {
			current = current.WithShootName(p.ShootNameFlag)
		}

		if err := current.Validate(); err != nil {
			return nil, fmt.Errorf("invalid target flags: %w", err)
		}
	}

	return current, nil
}

// Write takes a target and saves it permanently.
func (p *DynamicTargetProvider) Write(t target.Target) error {
	if p.hasCLIFlags() {
		return errors.New("cannot update target when using command-line flags for targeting")
	}

	return target.NewFilesystemTargetProvider(p.TargetFile).Write(t)
}
