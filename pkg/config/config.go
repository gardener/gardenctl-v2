/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/
package config

import (
	"fmt"
	"os"
	"regexp"

	"github.com/mitchellh/go-homedir"
	"gopkg.in/yaml.v3"
)

// Config holds the gardenctl configuration
type Config struct {
	// Gardens is a list of known Garden clusters
	Gardens []Garden `yaml:"gardens"`
	// MatchPatterns is a list of regex patterns that can be defined to use custom input formats for targeting
	// Use named capturing groups to match target values.
	// Supported capturing groups: garden, project, namespace, shoot
	MatchPatterns []string `yaml:"matchPatterns"`
}

// Garden represents one garden cluster
type Garden struct {
	// Name is a unique identifier of this Garden
	// The value is considered when evaluating the garden matcher pattern
	Name string `yaml:"name"`
	// Kubeconfig holds the path for the kubeconfig of the garden cluster
	Kubeconfig string `yaml:"kubeconfig"`
	// Aliases is a list of alternative names that can be used to target this Garden
	// Each value is considered when evaluating the garden matcher pattern
	Aliases []string `yaml:"aliases"`
}

// LoadFromFile parses a gardenctl config file and returns a Config struct
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

		// be nice and handle ~ in paths
		for i, g := range config.Gardens {
			expanded, err := homedir.Expand(g.Kubeconfig)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve ~ in kubeconfig path: %w", err)
			}

			config.Gardens[i].Kubeconfig = expanded
		}
	}

	return config, nil
}

// SaveToFile updates a gardenctl config file with the values passed via Config struct
func (config *Config) SaveToFile(filename string) error {
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

// TargetMatch represents a pattern match
type TargetMatch struct {
	// Garden is the matched Garden
	Garden string
	// Project is the matched Project
	Project string
	// Namespace is the matched Namespace, can be used to find the related project
	Namespace string
	// Shoot is the matched Shoot
	Shoot string
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}

	return false
}

// FindGarden returns name of Garden with provided name or first that includes the provided name in the list of aliases
func (config *Config) FindGarden(nameOrAlias string) (string, error) {
	for _, g := range config.Gardens {
		if g.Name == nameOrAlias {
			return g.Name, nil
		}

		if contains(g.Aliases, nameOrAlias) {
			return g.Name, nil
		}
	}

	return "", fmt.Errorf("garden with name or alias %q is not defined in gardenctl configuration", nameOrAlias)
}

const (
	patternGarden    = "garden"
	patternProject   = "project"
	patternNamespace = "namespace"
	patternShoot     = "shoot"
)

// MatchPattern matches a string against patterns defined in gardenctl config
// If matched, the function creates and returns a TargetMatch from the provided target string
func (config *Config) MatchPattern(value string) (*TargetMatch, error) {
	for _, p := range config.MatchPatterns {
		r, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("failed to compile configured regular expression %q: %v", p, err)
		}

		names := r.SubexpNames()
		matches := r.FindStringSubmatch(value)

		if matches == nil {
			continue
		}

		tm := &TargetMatch{}

		for i, name := range names {
			switch name {
			case patternGarden:
				tm.Garden = matches[i]
			case patternProject:
				tm.Project = matches[i]
			case patternNamespace:
				tm.Namespace = matches[i]
			case patternShoot:
				tm.Shoot = matches[i]
			}
		}

		return tm, nil
	}

	return nil, nil
}
