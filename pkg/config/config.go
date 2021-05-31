/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// Gardens is a list of known Garden clusters
	Gardens []Garden `yaml:"gardens"`
	// CloneKubeconfigs controls whether gardenctl will create
	// a copy of the canonical kubeconfig for a target. This makes
	// it easier to have context-based settings (think of multiple
	// shell instances, each having a clone of a shoot's kubeconfig,
	// but with different namespaces set in each of them), but
	// leads to orphaned temporary kubeconfig files that need
	// cleanup.
	CloneKubeconfigs bool `yaml:"cloneKubeconfigs"`
}

type Garden struct {
	Name       string `yaml:"name"`
	Kubeconfig string `yaml:"kubeconfig"`
}

func LoadFromFile(filename string) (*Config, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to determine filesize: %w", err)
	}

	config := &Config{}

	if stat.Size() > 0 {
		if err := yaml.NewDecoder(f).Decode(config); err != nil {
			return nil, fmt.Errorf("failed to decode as YAML: %w", err)
		}
	}

	return config, nil
}

func SaveToFile(filename string, config *Config) error {
	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if err := yaml.NewEncoder(f).Encode(config); err != nil {
		return fmt.Errorf("failed to encode as YAML: %w", err)
	}

	return nil
}
