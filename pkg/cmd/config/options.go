/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/component-base/cli/flag"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/config"
)

// Options is a struct to support all config subcommands
type options struct {
	base.Options
	// Configuration is the gardenctl configuration
	Configuration *config.Config
	// Command is the name of the config command
	Command string
	// Name is a unique name of this Garden that can be used to target this Garden
	Name string
	// KubeconfigFlag is the path to the kubeconfig file of the Garden cluster
	KubeconfigFlag flag.StringFlag
	// ContextFlag Overrides the current-context of the garden cluster kubeconfig
	// +optional
	ContextFlag flag.StringFlag
	// Patterns is a list of regex patterns that can be defined to use custom input formats for targeting
	// Use named capturing groups to match target values.
	// Supported capturing groups: project, namespace, shoot
	// +optional
	Patterns []string
}

// Complete adapts from the command line args to the data required.
func (o *options) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	manager, err := f.Manager()
	if err != nil {
		return fmt.Errorf("failed to get target manager: %w", err)
	}

	config := manager.Configuration()
	if config == nil {
		return errors.New("failed to get configuration")
	}

	o.Configuration = config

	switch o.Command {
	case "set-garden", "delete-garden":
		if len(args) > 0 {
			o.Name = strings.TrimSpace(args[0])
		}
	case "view":
		if o.Output == "" {
			o.Output = "yaml"
		}
	}

	return nil
}

// Validate validates the provided options
func (o *options) Validate() error {
	switch o.Command {
	case "set-garden", "delete-garden":
		if o.Name == "" {
			return errors.New("garden identity is required")
		}
	case "view":
		return o.Options.Validate()
	}

	return nil
}

// AddFlags adds flags to adjust the output to a cobra command
func (o *options) AddFlags(flags *pflag.FlagSet) {
	switch o.Command {
	case "set-garden":
		flags.Var(&o.KubeconfigFlag, "kubeconfig", "path to kubeconfig file for this Garden cluster")
		flags.Var(&o.ContextFlag, "context", "override the current-context of the garden cluster kubeconfig")
		flags.StringArrayVar(&o.Patterns, "pattern", nil, `define regex match patterns for this garden for custom input formats for targeting.
Use named capturing groups to match target values.
Supported capturing groups: project, namespace, shoot.
Note that if you set this flag it will overwrite the pattern list in the config file.
You may specify any number of extra patterns.
Example: ^((?Pmy-garden[^/]+)/)?shoot--(?P<project>.+)--(?P<shoot>.+)$`)
	case "delete-garden":
		// no flags
	case "view":
		o.Options.AddFlags(flags)
	}
}

// Run executes the command
func (o *options) Run(_ util.Factory) error {
	switch o.Command {
	case "set-garden":
		return o.runSetGarden()
	case "delete-garden":
		return o.runDeleteGarden()
	case "view":
		return o.runView()
	}

	return nil
}

func (o *options) runSetGarden() error {
	garden, err := o.Configuration.Garden(o.Name)
	if err == nil {
		if o.KubeconfigFlag.Provided() {
			garden.Kubeconfig = o.KubeconfigFlag.Value()
		}

		if o.ContextFlag.Provided() {
			garden.Context = o.ContextFlag.Value()
		}

		if o.Patterns != nil {
			firstPattern := o.Patterns[0]
			if len(firstPattern) > 0 {
				garden.Patterns = o.Patterns
			} else {
				garden.Patterns = []string{}
			}
		}
	} else {
		o.Configuration.Gardens = append(o.Configuration.Gardens, config.Garden{
			Name:       o.Name,
			Kubeconfig: o.KubeconfigFlag.Value(),
			Context:    o.ContextFlag.Value(),
			Patterns:   o.Patterns,
		})
	}

	err = o.Configuration.Save()
	if err != nil {
		return fmt.Errorf("failed to configure garden: %w", err)
	}

	fmt.Fprintf(o.IOStreams.Out, "Successfully configured garden %q\n", o.Name)

	return nil
}

func (o *options) runDeleteGarden() error {
	err := o.Configuration.DeleteGarden(o.Name)
	if err != nil {
		return fmt.Errorf("failed to delete garden from configuration: %w", err)
	}

	fmt.Fprintf(o.IOStreams.Out, "Successfully deleted garden %q\n", o.Name)

	return nil
}

func (o *options) runView() error {
	return o.PrintObject(o.Configuration)
}
