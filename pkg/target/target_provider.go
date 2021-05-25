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

// TargetProvider can read and write targets.
type TargetProvider interface {
	// Read returns the current target.
	Read() (Target, error)
	// Write takes a target and saves it permanently.
	Write(t Target) error
}

type fsTargetProvider struct {
	targetFile string
}

var _ TargetProvider = &fsTargetProvider{}

// NewFilesystemTargetProvider returns a new TargetProvider that
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
