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

type Config struct {
	// Gardens is a list of known Garden clusters
	Gardens       []Garden `yaml:"gardens"`
	MatchPatterns []string `yaml:"matchPatterns"`
}

type Garden struct {
	Name       string   `yaml:"name"`
	Kubeconfig string   `yaml:"kubeconfig"`
	Aliases    []string `yaml:"aliases"`
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

type TargetMatch struct {
	Garden    string
	Project   string
	Namespace string
	Shoot     string
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}

	return false
}

func (config *Config) FindGarden(nameOrAlias string) string {
	for _, g := range config.Gardens {
		if g.Name == nameOrAlias {
			return g.Name
		}

		if contains(g.Aliases, nameOrAlias) {
			return g.Name
		}
	}

	return ""
}

// MatchPattern matches a string against patterns defined in gardenctl config
// If matched, the function creates and returns a TargetMatch from the provided target string
func (config *Config) MatchPattern(value string) (*TargetMatch, error) {
	for _, p := range config.MatchPatterns {
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

		return tm, nil
	}

	return nil, nil
}
