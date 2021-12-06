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

	"k8s.io/component-base/cli/flag"

	"github.com/mitchellh/go-homedir"
	"gopkg.in/yaml.v3"
)

// Config holds the gardenctl configuration
type Config struct {
	// Gardens is a list of known Garden clusters
	Gardens []Garden `yaml:"gardens"`
}

// Garden represents one garden cluster
type Garden struct {
	// ClusterIdentity is a unique identifier of this Garden that can be used to target this Garden
	ClusterIdentity string `yaml:"clusterIdentifier"`
	// Kubeconfig holds the path for the kubeconfig of the garden cluster
	Kubeconfig string `yaml:"kubeconfig"`
	// Context if set, context overwrites the current-context of the cluster kubeconfig
	Context string `yaml:"context"`
	// Name is an alternative name that can be used to target this Garden
	Name string `yaml:"name"`
	// MatchPatterns is a list of regex patterns that can be defined to use custom input formats for targeting
	// Use named capturing groups to match target values.
	// Supported capturing groups: project, namespace, shoot
	MatchPatterns []string `yaml:"matchPatterns"`
}

// TargetName returns the name of the Garden which is either the cluster identity or, if configured, the cluster name
func (garden *Garden) TargetName() string {
	if garden.Name != "" {
		return garden.Name
	}

	return garden.ClusterIdentity
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

// PatternMatch holds (target) values extracted from a provided string
type PatternMatch struct {
	// Garden is the matched Garden
	Garden string
	// Project is the matched Project
	Project string
	// Namespace is the matched Namespace, can be used to find the related project
	Namespace string
	// Shoot is the matched Shoot
	Shoot string
}

// Garden returns a Garden cluster from the list of configured Gardens
func (config *Config) Garden(name string) (*Garden, error) {
	for _, g := range config.Gardens {
		if g.ClusterIdentity == name || g.TargetName() == name {
			return &g, nil
		}
	}

	return nil, fmt.Errorf("garden with identity or name %q is not defined in gardenctl configuration", name)
}

// PatternKey is a key that can be used to identify a value in a pattern
type PatternKey string

const (
	// PatternKeyProject is used to identify a Project
	PatternKeyProject = PatternKey("project")
	// PatternKeyNamespace is used to identify a Project by the namespace it refers to
	PatternKeyNamespace = PatternKey("namespace")
	// PatternKeyShoot is used to identify a Shoot
	PatternKeyShoot = PatternKey("shoot")
)

// MatchPattern matches a string against patterns defined in gardenctl config
// If matched, the function creates and returns a PatternMatch from the provided target string
func (config *Config) MatchPattern(value string, currentGarden string) (*PatternMatch, error) {
	var patternMatch *PatternMatch

	for _, g := range config.Gardens {
		match, err := matchPattern(g.MatchPatterns, value)

		if err != nil {
			return nil, err
		}

		if match != nil {
			match.Garden = g.TargetName()

			if currentGarden == "" || g.TargetName() == currentGarden {
				// Directly return match of selected garden
				return match, nil
			}

			patternMatch = match
		}
	}

	if patternMatch != nil {
		// Did not match pattern of current garden, but did match other pattern
		return patternMatch, nil
	}

	return nil, errors.New("the provided value does not match any pattern")
}

// matchPattern matches pattern with provided list of patterns
func matchPattern(patterns []string, value string) (*PatternMatch, error) {
	for _, p := range patterns {
		r, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("failed to compile configured regular expression %q: %w", p, err)
		}

		names := r.SubexpNames()
		matches := r.FindStringSubmatch(value)

		if matches == nil {
			continue
		}

		tm := &PatternMatch{}

		for i, name := range names {
			switch PatternKey(name) {
			case PatternKeyProject:
				tm.Project = matches[i]
			case PatternKeyNamespace:
				tm.Namespace = matches[i]
			case PatternKeyShoot:
				tm.Shoot = matches[i]
			}
		}

		return tm, nil
	}

	return nil, nil
}

// SetGarden adds or updates a Garden in the configuration
func (config *Config) SetGarden(clusterIdentity string, kubeconfigFile flag.StringFlag, contextName flag.StringFlag, name flag.StringFlag, patterns []string, configFilename string) error {
	var garden *Garden

	for i, g := range config.Gardens {
		if g.ClusterIdentity == clusterIdentity {
			garden = &config.Gardens[i]
			break
		}
	}

	if garden != nil {
		// update existing garden in configuration
		if kubeconfigFile.Provided() {
			garden.Kubeconfig = kubeconfigFile.Value()
		}

		if contextName.Provided() {
			garden.Context = contextName.Value()
		}

		if name.Provided() {
			garden.Name = name.Value()
		}

		if patterns != nil {
			if len(patterns[0]) > 0 {
				garden.MatchPatterns = patterns
			} else {
				garden.MatchPatterns = []string{}
			}
		}
	} else {
		newGarden := Garden{
			ClusterIdentity: clusterIdentity,
			Kubeconfig:      kubeconfigFile.Value(),
			Context:         contextName.Value(),
			Name:            name.Value(),
			MatchPatterns:   patterns,
		}

		config.Gardens = append(config.Gardens, newGarden)
	}

	return config.SaveToFile(configFilename)
}

// DeleteGarden deletes a Garden from the configuration
func (config *Config) DeleteGarden(clusterIdentity string, configFilename string) error {
	var newGardens []Garden

	for _, g := range config.Gardens {
		if g.ClusterIdentity != clusterIdentity {
			newGardens = append(newGardens, g)
		}
	}

	if len(config.Gardens) == len(newGardens) {
		return fmt.Errorf("failed to delete garden with cluster identity %q", clusterIdentity)
	}

	config.Gardens = newGardens

	return config.SaveToFile(configFilename)
}
