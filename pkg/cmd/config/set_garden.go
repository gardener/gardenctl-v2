/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/component-base/cli/flag"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/config"
)

// NewCmdConfigSetGarden returns a new (config) set-garden command.
func NewCmdConfigSetGarden(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := &setGardenOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
	cmd := &cobra.Command{
		Use:               "set-garden",
		Short:             "modify or add Garden to gardenctl configuration",
		Long:              "Modify or add Garden to gardenctl configuration. E.g. \"gardenctl config set-garden my-garden --kubeconfig ~/.kube/kubeconfig.yaml\" to configure or add a garden with identity 'my-garden'",
		ValidArgsFunction: validGardenArgsFunctionWrapper(f, ioStreams),
		RunE:              base.WrapRunE(o, f),
	}

	o.AddFlags(cmd.Flags())

	return cmd
}

type setGardenOptions struct {
	base.Options
	// Configuration is the gardenctl configuration
	Configuration *config.Config
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
func (o *setGardenOptions) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	config, err := getConfiguration(f)
	if err != nil {
		return err
	}

	o.Configuration = config

	if len(args) > 0 {
		o.Name = strings.TrimSpace(args[0])
	}

	return nil
}

// Validate validates the provided options
func (o *setGardenOptions) Validate() error {
	if o.Name == "" {
		return errors.New("garden identity is required")
	}

	if err := validatePatterns(o.Patterns); err != nil {
		return err
	}

	return nil
}

// AddFlags adds flags to adjust the output to a cobra command
func (o *setGardenOptions) AddFlags(flags *pflag.FlagSet) {
	flags.Var(&o.KubeconfigFlag, "kubeconfig", "path to kubeconfig file for this Garden cluster")
	flags.Var(&o.ContextFlag, "context", "override the current-context of the garden cluster kubeconfig")
	flags.StringArrayVar(&o.Patterns, "pattern", nil, `define regex match patterns for this garden for custom input formats for targeting.
Use named capturing groups to match target values.
Supported capturing groups: project, namespace, shoot.
Note that if you set this flag it will overwrite the pattern list in the config file.
You may specify any number of extra patterns.
Example: ^(?:my-garden/)?shoot--(?P<project>.+)--(?P<shoot>.+)$`)
}

// Run executes the command
func (o *setGardenOptions) Run(_ util.Factory) error {
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
				garden.Patterns = nil
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

func validatePatterns(patterns []string) error {
	if patterns == nil || patterns[0] == "" && len(patterns) == 1 {
		return nil
	}

	for i, p := range patterns {
		if p == "" {
			return fmt.Errorf("pattern[%d] must not be empty", i)
		}

		re, err := regexp.Compile(p)
		if err != nil {
			return fmt.Errorf("pattern[%d] is not a valid regular expression: %w", i, err)
		}

		names := re.SubexpNames()
		for _, name := range names[1:] {
			if name != "project" && name != "namespace" && name != "shoot" {
				return fmt.Errorf("pattern[%d] contains an invalid subexpression %q", i, name)
			}
		}
	}

	return nil
}
