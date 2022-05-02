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
	"strconv"

	"github.com/mitchellh/go-homedir"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// Config holds the gardenctl configuration
type Config struct {
	// Filename is the name of the gardenctl configuration file
	Filename string `yaml:"-" json:"-"`
	// LinkKubeconfig defines if kubeconfig is symlinked with the target
	LinkKubeconfig *bool `yaml:"linkKubeconfig,omitempty" json:"linkKubeconfig,omitempty"`
	// Gardens is a list of known Garden clusters
	Gardens []Garden `yaml:"gardens" json:"gardens"`
}

// Garden represents one garden cluster
type Garden struct {
	// Name is a unique identifier of this Garden that can be used to target this Garden
	Name string `yaml:"identity" json:"identity"`
	// Kubeconfig holds the path for the kubeconfig of the garden cluster
	Kubeconfig string `yaml:"kubeconfig" json:"kubeconfig"`
	// Context overrides the current-context of the garden cluster kubeconfig
	// +optional
	Context string `yaml:"context,omitempty" json:"context,omitempty"`
	// Patterns is a list of regex patterns that can be defined to use custom input formats for targeting
	// Use named capturing groups to match target values.
	// Supported capturing groups: project, namespace, shoot
	// +optional
	Patterns []string `yaml:"patterns,omitempty" json:"patterns,omitempty"`
}

// LoadFromFile parses a gardenctl config file and returns a Config struct
func LoadFromFile(filename string) (*Config, error) {
	config := &Config{Filename: filename}

	f, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}

		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to determine filesize: %w", err)
	}

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

	// we don't want a dependency to root command here
	str, ok := os.LookupEnv("GCTL_LINK_KUBECONFIG")
	if ok {
		val, err := strconv.ParseBool(str)
		if err != nil {
			return nil, fmt.Errorf("failed to parse environment variable GCTL_LINK_KUBECONFIG: %w", err)
		}

		config.LinkKubeconfig = &val
	}

	return config, nil
}

// SymlinkTargetKubeconfig indicates if the kubeconfig of the current target should be always symlinked
func (config *Config) SymlinkTargetKubeconfig() bool {
	return config.LinkKubeconfig == nil || *config.LinkKubeconfig
}

// Save updates a gardenctl config file with the values passed via Config struct
func (config *Config) Save() error {
	f, err := os.OpenFile(config.Filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if err := yaml.NewEncoder(f).Encode(config); err != nil {
		return fmt.Errorf("failed to encode as YAML: %w", err)
	}

	return nil
}

// IndexOfGarden returns the index of the Garden with the given name in the configured Gardens slice
// If no Garden with this name is found it returns -1
func (config *Config) IndexOfGarden(name string) (int, bool) {
	for i, g := range config.Gardens {
		if g.Name == name {
			return i, true
		}
	}

	return -1, false
}

// GardenNames returns a slice containing the names of the configured Gardens
func (config *Config) GardenNames() []string {
	names := []string{}
	for _, g := range config.Gardens {
		names = append(names, g.Name)
	}

	return names
}

// Garden returns a Garden cluster from the list of configured Gardens
func (config *Config) Garden(name string) (*Garden, error) {
	i, ok := config.IndexOfGarden(name)
	if !ok {
		return nil, fmt.Errorf("garden %q is not defined in gardenctl configuration", name)
	}

	return &config.Gardens[i], nil
}

// ClientConfig returns a deferred loading client config for a configured garden cluster
func (config *Config) ClientConfig(name string) (clientcmd.ClientConfig, error) {
	garden, err := config.Garden(name)
	if err != nil {
		return nil, err
	}

	loader := &clientcmd.ClientConfigLoadingRules{ExplicitPath: garden.Kubeconfig}

	overrides := &clientcmd.ConfigOverrides{}
	if garden.Context != "" {
		overrides.CurrentContext = garden.Context
	}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loader, overrides), nil
}

// DirectClientConfig returns a directly loaded client config for a configured garden cluster
func (config *Config) DirectClientConfig(name string) (clientcmd.ClientConfig, error) {
	garden, err := config.Garden(name)
	if err != nil {
		return nil, err
	}

	rawConfig, err := garden.LoadRawConfig()
	if err != nil {
		return nil, err
	}

	return clientcmd.NewDefaultClientConfig(*rawConfig, nil), nil
}

//LoadRawConfig directly loads the raw config from file, validates the content and removes all the irrelevant pieces
func (g *Garden) LoadRawConfig() (*clientcmdapi.Config, error) {
	rawConfig, err := clientcmd.LoadFromFile(g.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load client configuration: %w", err)
	}

	err = clientcmd.Validate(*rawConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to validate client configuration: %w", err)
	}

	if g.Context != "" {
		rawConfig.CurrentContext = g.Context
	}

	// this function returns an error if the currentContext does not exist
	err = clientcmdapi.MinifyConfig(rawConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to minify client configuration: %w", err)
	}

	return rawConfig, nil
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
func (config *Config) MatchPattern(preferredGardenName string, value string) (*PatternMatch, error) {
	if preferredGardenName != "" {
		g, err := config.Garden(preferredGardenName)
		if err != nil {
			return nil, err
		}

		match, err := matchPattern(g.Patterns, value)
		if err != nil {
			return nil, err
		}

		if match != nil {
			match.Garden = g.Name
			return match, nil
		}
	}

	var patternMatch *PatternMatch

	for _, g := range config.Gardens {
		match, err := matchPattern(g.Patterns, value)
		if err != nil {
			return nil, err
		}

		if match != nil {
			match.Garden = g.Name

			if patternMatch != nil {
				return nil, errors.New("the provided value resulted in an ambiguous match")
			}

			patternMatch = match
		}
	}

	if patternMatch == nil {
		return nil, errors.New("the provided value does not match any pattern")
	}

	return patternMatch, nil
}

// matchPattern matches pattern with provided list of patterns
// If none of the provided patterns matches the given value no error is returned
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
