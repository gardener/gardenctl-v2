/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/
package target

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// TargetProvider can read and write targets.
//nolint
type TargetProvider interface {
	// Read returns the current target. If no target exists yet, a default
	// (empty) target is returned.
	Read() (Target, error)
	// Write takes a target and saves it permanently.
	Write(t Target) error
}

type fsTargetProvider struct {
	targetFile string
}

var _ TargetProvider = &fsTargetProvider{}

// NewFilesystemTargetProvider returns a new Provider that
// reads and writes from the local filesystem.
func NewFilesystemTargetProvider(targetFile string) TargetProvider {
	return &fsTargetProvider{
		targetFile: targetFile,
	}
}

// Read returns the current target.
func (p *fsTargetProvider) Read() (Target, error) {
	f, err := os.Open(p.targetFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &targetImpl{}, nil
		}

		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to determine filesize: %w", err)
	}

	target := &targetImpl{}
	if stat.Size() > 0 {
		if err := yaml.NewDecoder(f).Decode(target); err != nil {
			return nil, fmt.Errorf("failed to decode as YAML: %w", err)
		}
	}

	if err := target.Validate(); err != nil {
		return nil, fmt.Errorf("target is invalid: %w", err)
	}

	return target, nil
}

// Write takes a target and saves it permanently.
func (p *fsTargetProvider) Write(t Target) error {
	f, err := os.OpenFile(p.targetFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if err := yaml.NewEncoder(f).Encode(t); err != nil {
		return fmt.Errorf("failed to encode as YAML: %w", err)
	}

	return nil
}

// DynamicTargetProvider is a wrapper that combines the basic
// file-based TargetProvider with CLI flags, to allow the user
// to change the target for individual gardenctl commands
// on-the-fly without changing the file on disk every time.
//
// If no CLI flags are given, this functions identical to the
// regular TargetProvider from NewFilesystemTargetProvider().
//
// Otherwise, the flags are used to augment the existing target.
type DynamicTargetProvider struct {
	// TargetFile is the file where the target is read from / written to
	// when no CLI flags override the targeting. If this is empty, no file
	// will be loaded as a fallback.
	TargetFile string

	// GardenNameFlag is the value that should be tied to a cobra flag.
	GardenNameFlag string
	// ProjectNameFlag is the value that should be tied to a cobra flag.
	ProjectNameFlag string
	// SeedNameFlag is the value that should be tied to a cobra flag.
	SeedNameFlag string
	// ShootNameFlag is the value that should be tied to a cobra flag.
	ShootNameFlag string
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

	return NewTarget(p.GardenNameFlag, p.ProjectNameFlag, p.SeedNameFlag, p.ShootNameFlag).Validate() == nil
}

// Read returns the current target from the TargetFile if no CLI
// flags were given, and tries to construct a meaningful target
// otherwise.
func (p *DynamicTargetProvider) Read() (Target, error) {
	// user gave everything we needed
	if p.hasCompleteCLIFlags() {
		return NewTarget(p.GardenNameFlag, p.ProjectNameFlag, p.SeedNameFlag, p.ShootNameFlag), nil
	}

	// user didn't specify anything at all or _some_ flags; in both
	// cases we need to read the current target from disk
	var current Target

	if p.TargetFile != "" {
		var err error

		current, err = NewFilesystemTargetProvider(p.TargetFile).Read()
		if err != nil {
			return nil, err
		}
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
func (p *DynamicTargetProvider) Write(t Target) error {
	return NewFilesystemTargetProvider(p.TargetFile).Write(t)
}
