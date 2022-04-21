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

//go:generate mockgen -destination=./mocks/mock_target_trovider.go -package=mocks github.com/gardener/gardenctl-v2/pkg/target TargetProvider

// TargetProvider can read and write targets.
//nolint
type TargetProvider interface {
	TargetReader
	TargetWriter
}

// fsTargetProvider is a TragetProvider that
// reads and writes from the local filesystem.
type fsTargetProvider struct {
	targetFile string
}

var _ TargetProvider = &fsTargetProvider{}

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

// NewTargetProvider returns a new TargetProvider that
// reads and writes the current Target.
func NewTargetProvider(targetFile string, targetFlags TargetFlags) TargetProvider {
	delegate := &fsTargetProvider{
		targetFile: targetFile,
	}

	if targetFlags == nil {
		return delegate
	}

	return &dynamicTargetProvider{
		delegate:    delegate,
		targetFlags: targetFlags,
	}
}

// dynamicTargetProvider is a wrapper that combines the basic
// filesystem based TargetProvider with CLI flags, to allow the user
// to change the target for individual gardenctl commands
// on-the-fly without changing the file on disk every time.
//
// If no CLI flags are given, this functions identical to the
// regular TargetProvider from NewFilesystemTargetProvider().
//
// Otherwise, the flags are used to augment the existing target.
type dynamicTargetProvider struct {
	// delegate must be valid a filesystem based TargetProvider (required)
	delegate *fsTargetProvider
	// targetFlags refers to the global target CLI flags (required)
	targetFlags TargetFlags
}

var _ TargetProvider = &dynamicTargetProvider{}

// Read returns the current target from the TargetFile if no CLI
// flags were given, and tries to construct a meaningful target
// otherwise.
func (p *dynamicTargetProvider) Read() (Target, error) {
	// user gave everything we needed
	if p.targetFlags.IsTargetValid() {
		return p.targetFlags.ToTarget(), nil
	}

	// user didn't specify anything at all or _some_ flags;
	// in both cases we need to read the current target from disk
	current, err := p.delegate.Read()
	if err != nil {
		return nil, err
	}

	return p.targetFlags.OverrideTarget(current)
}

// Write takes a target and saves it permanently.
func (p *dynamicTargetProvider) Write(t Target) error {
	return p.delegate.Write(t)
}
