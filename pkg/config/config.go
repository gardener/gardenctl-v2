/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/mitchellh/go-homedir"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"

	"github.com/gardener/gardenctl-v2/pkg/ac"
	"github.com/gardener/gardenctl-v2/pkg/provider/common/allowpattern"
	"github.com/gardener/gardenctl-v2/pkg/provider/credvalidate"
)

// Config holds the gardenctl configuration.
type Config struct {
	// Filename is the name of the gardenctl configuration file
	Filename string `json:"-"`
	// LinkKubeconfig defines if kubeconfig is symlinked with the target
	LinkKubeconfig *bool `json:"linkKubeconfig,omitempty"`
	// Gardens is a list of known Garden clusters
	Gardens []Garden `json:"gardens"`
	// Provider holds provider-specific configuration
	Provider *ProviderConfig `json:"provider,omitempty"`
}

// Garden represents one garden cluster.
type Garden struct {
	// Name is a unique identifier of this Garden that can be used to target this Garden
	Name string `json:"identity"`
	// Alias is a unique identifier of this Garden that can be used as an alternate name to target this Garden
	// +optional
	Alias string `json:"name,omitempty"`
	// Kubeconfig holds the path for the kubeconfig of the garden cluster
	Kubeconfig string `json:"kubeconfig"`
	// Context overrides the current-context of the garden cluster kubeconfig
	// +optional
	Context string `json:"context,omitempty"`
	// Patterns is a list of regex patterns that can be defined to use custom input formats for targeting
	// Use named capturing groups to match target values.
	// Supported capturing groups: project, namespace, shoot
	// +optional
	Patterns []string `json:"patterns,omitempty"`
	// AccessRestrictions is a list of access restriction definitions
	// +optional
	AccessRestrictions []ac.AccessRestriction `json:"accessRestrictions,omitempty"`
}

// ProviderConfig represents provider-specific configuration options.
type ProviderConfig struct {
	// OpenStack configuration options
	OpenStack *OpenStackConfig `json:"openstack,omitempty"`
	// STACKIT configuration options
	STACKIT *STACKITConfig `json:"stackit,omitempty"`
}

// OpenStackConfig represents OpenStack-specific configuration options.
type OpenStackConfig struct {
	// AllowedPatterns is a list of allowed patterns for OpenStack credential fields.
	AllowedPatterns []allowpattern.Pattern `json:"allowedPatterns,omitempty"`
}

// STACKITConfig represents STACKIT-specific configuration options.
type STACKITConfig struct {
	// AllowedPatterns is a list of allowed patterns for OpenStack credential fields.
	AllowedPatterns []allowpattern.Pattern `json:"allowedPatterns,omitempty"`
}

// LoadFromFile parses a gardenctl config file and returns a Config struct.
func LoadFromFile(filename string) (*Config, error) {
	config := &Config{Filename: filename}

	f, err := os.Open(filename) // #nosec G304 -- Accepting user-provided config file path by design
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
		buf, err := os.ReadFile(filename) // #nosec G304 -- Accepting user-provided config file path by design
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err = yaml.Unmarshal(buf, config); err != nil {
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

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

// Validate validates the entire config.
func (config *Config) Validate() error {
	// Validate gardens
	if err := config.validateGardens(); err != nil {
		return err
	}

	// Validate provider config
	if config.Provider != nil {
		if config.Provider.OpenStack != nil {
			if err := config.Provider.OpenStack.Validate(); err != nil {
				return fmt.Errorf("invalid OpenStack provider configuration: %w", err)
			}
		}
	}

	return nil
}

// Validate validates the OpenStack configuration.
func (o *OpenStackConfig) Validate() error {
	for i, pattern := range o.AllowedPatterns {
		if err := pattern.ValidateWithContext(credvalidate.GetOpenStackValidationContext()); err != nil {
			return fmt.Errorf("invalid allowed pattern at index %d: %w", i, err)
		}
	}

	return nil
}

// validateGardens checks the config for valid garden names and ambiguous definitions.
func (config *Config) validateGardens() error {
	seen := make(map[string]bool, len(config.Gardens))

	for i := range config.Gardens {
		garden := config.Gardens[i]

		if garden.Name == "" {
			return fmt.Errorf("garden at index %d has an empty name", i)
		}

		if err := ValidateGardenName(garden.Name); err != nil {
			return fmt.Errorf("invalid garden name %q: %w", garden.Name, err)
		}

		if garden.Alias != "" {
			if err := ValidateGardenName(garden.Alias); err != nil {
				return fmt.Errorf("invalid garden alias %q for garden %q: %w", garden.Alias, garden.Name, err)
			}
		}

		if logged, ok := seen[garden.Name]; ok && !logged {
			klog.Warningf("identity and alias should be unique but %q was found multiple times in gardenctl configuration", garden.Name)
			seen[garden.Name] = true
		} else if !ok {
			seen[garden.Name] = false
		}

		if garden.Alias != "" && garden.Alias != garden.Name {
			if logged, ok := seen[garden.Alias]; ok && !logged {
				klog.Warningf("identity and alias should be unique but %q was found multiple times in gardenctl configuration", garden.Alias)

				seen[garden.Alias] = true
			} else if !ok {
				seen[garden.Alias] = false
			}
		}
	}

	return nil
}

// SymlinkTargetKubeconfig indicates if the kubeconfig of the current target should be always symlinked.
func (config *Config) SymlinkTargetKubeconfig() bool {
	return config.LinkKubeconfig == nil || *config.LinkKubeconfig
}

// Save updates a gardenctl config file with the values passed via Config struct.
func (config *Config) Save() error {
	dir := filepath.Dir(config.Filename)

	err := os.MkdirAll(dir, 0o700)
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	buf, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to encode as YAML: %w", err)
	}

	if err := os.WriteFile(config.Filename, buf, 0o600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// IndexOfGarden returns the index of the Garden with the given name in the configured Gardens slice.
// If no Garden with this name is found it returns -1.
func (config *Config) IndexOfGarden(name string) (int, bool) {
	for i, g := range config.Gardens {
		if g.Name == name {
			return i, true
		}
	}

	return -1, false
}

// GardenNames returns a slice containing the names of the configured Gardens.
func (config *Config) GardenNames() []string {
	names := []string{}
	for _, g := range config.Gardens {
		names = append(names, g.Name)
	}

	return names
}

// Garden returns the matching Garden cluster by name (identity or alias) from the list
// of configured Gardens. In case of ambigous names the first match is returned and identity is
// preferred over alias.
func (config *Config) Garden(name string) (*Garden, error) {
	if name == "" {
		return nil, fmt.Errorf("garden name or alias cannot be empty")
	}

	var firstMatchByAlias *Garden

	for idx := range config.Gardens {
		cfg := &config.Gardens[idx]
		if name == cfg.Name {
			return cfg, nil
		}

		if firstMatchByAlias == nil && name == cfg.Alias {
			firstMatchByAlias = cfg
		}
	}

	if firstMatchByAlias != nil {
		return firstMatchByAlias, nil
	}

	return nil, fmt.Errorf("garden %q is not defined in gardenctl configuration", name)
}

// ClientConfig returns a deferred loading client config for a configured garden cluster.
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

// DirectClientConfig returns a directly loaded client config for a configured garden cluster.
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

// LoadRawConfig directly loads the raw config from file, validates the content and removes all the irrelevant pieces.
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

// PatternMatch holds (target) values extracted from a provided string.
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

// PatternKey is a key that can be used to identify a value in a pattern.
type PatternKey string

const (
	// PatternKeyProject is used to identify a Project.
	PatternKeyProject = PatternKey("project")
	// PatternKeyNamespace is used to identify a Project by the namespace it refers to.
	PatternKeyNamespace = PatternKey("namespace")
	// PatternKeyShoot is used to identify a Shoot.
	PatternKeyShoot = PatternKey("shoot")
)

// MatchPattern matches a string against patterns defined in gardenctl config.
// If matched, the function creates and returns a PatternMatch from the provided target string.
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

// matchPattern matches pattern with provided list of patterns.
// If none of the provided patterns matches the given value no error is returned.
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

var (
	// allowedCharsPattern checks if the string contains only alphanumeric characters, underscore or hyphen.
	allowedCharsPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	// startsAndEndsWithAlphanumericPattern checks if the string starts and ends with an alphanumeric character.
	startsAndEndsWithAlphanumericPattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9_-]*[a-zA-Z0-9])?$`)
)

// ValidateGardenName validates that a garden name follows the naming rules:
// 1. Must contain only alphanumeric characters, underscore or hyphen
// 2. Must start and end with an alphanumeric character.
func ValidateGardenName(name string) error {
	if !allowedCharsPattern.MatchString(name) {
		return errors.New("garden name must contain only alphanumeric characters, underscore or hyphen")
	}

	if !startsAndEndsWithAlphanumericPattern.MatchString(name) {
		return errors.New("garden name must start and end with an alphanumeric character")
	}

	return nil
}
