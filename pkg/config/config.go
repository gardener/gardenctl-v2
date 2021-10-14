/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/
package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"

	"github.com/mitchellh/go-homedir"
	"gopkg.in/yaml.v3"
)

type Config struct {
	// Gardens is a list of known Garden clusters
	Gardens       []Garden `yaml:"gardens"`
	MatchPatterns []string `yaml:"matchPatterns"`
}

type Garden struct {
	Name          string   `yaml:"name"`
	Kubeconfig    string   `yaml:"kubeconfig"`
	MatchPatterns []string `yaml:"matchPatterns"`
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

func (config *Config) ConfiguredPatternsForGarden(garden string) []string {
	patterns := config.MatchPatterns

	for _, g := range config.Gardens {
		for _, p := range g.MatchPatterns {
			if g.Name == garden {
				patterns = append(patterns, p)
			}
		}
	}

	return patterns
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

type TargetMatch struct {
	Garden    string
	Project   string
	Namespace string
	Shoot     string
}

func (tm TargetMatch) ValidMatch() error {
	if tm.Garden == "" {
		return errors.New("target match is incomplete: Garden is not set")
	}

	return nil
}

// MatchPattern matches a string against patterns defined in gardenctl config
// If matched, the function creates and returns a target from the provided target string
func (config *Config) MatchPattern(garden string, value string) (*TargetMatch, error) {
	for _, p := range config.ConfiguredPatternsForGarden(garden) {
		r, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("failed to compile configured regular expression: %v", err)
		}

		names := r.SubexpNames()
		matches := r.FindStringSubmatch(value)

		if matches == nil {
			continue
		}

		tm := &TargetMatch{}

		for i, name := range names {
			switch name {
			case "garden":
				tm.Garden = matches[i]
			case "project":
				tm.Project = matches[i]
			case "namespace":
				tm.Namespace = matches[i]
			case "shoot":
				tm.Shoot = matches[i]
			}
		}

		if tm.Garden == "" {
			tm.Garden = garden
		}

		return tm, nil
	}

	return nil, nil
}
