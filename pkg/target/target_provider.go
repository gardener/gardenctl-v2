/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/
package target

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// TargetReader can read targets.
//nolint
type TargetReader interface {
	// Read returns the current target. If no target exists yet, a default
	// (empty) target is returned.
	Read() (Target, error)
}

// TargetWriter can write targets.
//nolint
type TargetWriter interface {
	// Write takes a target and saves it permanently.
	Write(t Target) error
}

// TargetProvider can read and write targets.
//nolint
type TargetProvider interface {
	TargetReader
	TargetWriter
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

// NewDynamicTargetProvider returns a wrapper that combines the basic
// file-based TargetProvider with CLI flags, to allow the user
// to change the target for individual gardenctl commands
// on-the-fly without changing the file on disk every time.
//
// If no CLI flags are given, this functions identical to the
// regular TargetProvider from NewFilesystemTargetProvider().
//
// Otherwise, the flags are used to augment the existing target.
func NewDynamicTargetProvider(targetFile string, targetFlags TargetFlags) TargetProvider {
	var delegate TargetProvider

	if targetFile != "" {
		delegate = NewFilesystemTargetProvider(targetFile)
	}

	if targetFlags == nil {
		return delegate
	}

	return &dynamicTargetProvider{
		delegate:    delegate,
		targetFlags: targetFlags,
	}
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
type dynamicTargetProvider struct {
	delegate    TargetProvider
	targetFlags TargetFlags
}

var _ TargetProvider = &dynamicTargetProvider{}

// Read returns the current target from the TargetFile if no CLI
// flags were given, and tries to construct a meaningful target
// otherwise.
func (p *dynamicTargetProvider) Read() (Target, error) {
	// user gave everything we needed
	if p.targetFlags != nil && p.targetFlags.isTargetValid() {
		return p.targetFlags.ToTarget(), nil
	}

	// user didn't specify anything at all or _some_ flags; in both
	// cases we need to read the current target from disk
	current := NewTarget("", "", "", "")

	if p.delegate != nil {
		var err error

		current, err = p.delegate.Read()
		if err != nil {
			return nil, err
		}
	}

	// user gave _some_ flags; we use those to override the current target
	// (e.g. to quickly change a shoot while keeping garden/project names)
	if p.targetFlags != nil {
		// note that "deeper" levels of targets are reset, as to allow the
		// user to "move up", e.g. when they have targeted a shoot, just
		// specifying "--garden mygarden" should target the garden, not the same
		// shoot on the garden mygarden.
		return p.targetFlags.overrideTarget(current)
	}

	return current, nil
}

// Write takes a target and saves it permanently.
func (p *dynamicTargetProvider) Write(t Target) error {
	return p.delegate.Write(t)
}
