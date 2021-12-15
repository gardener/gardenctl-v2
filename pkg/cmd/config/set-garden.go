/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"errors"
	"fmt"
	"strings"

	"k8s.io/component-base/cli/flag"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"

	"github.com/spf13/cobra"
)

// NewCmdConfigSetGarden returns a new (config) set-garden command.
func NewCmdConfigSetGarden(f util.Factory, o *SetGardenOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-garden",
		Short: "modify or add Garden to gardenctl configuration",
		Long:  "Modify or add Garden to gardenctl configuration. E.g. \"gardenctl config set-garden my-garden --kubeconfig ~/.kube/kubeconfig.yaml\" to configure or add a garden with identity 'my-garden'",
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			suggestions, err := validGardenArgsFunction(f, args)
			if err != nil {
				fmt.Fprintln(o.IOStreams.ErrOut, err.Error())
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			return util.FilterStringsByPrefix(toComplete, suggestions), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: base.WrapRunE(o, f),
	}

	cmd.Flags().Var(&o.KubeconfigFile, "kubeconfig", "path to kubeconfig file for this Garden cluster. If used without --context, current-context of kubeconfig will be set as context")
	cmd.Flags().Var(&o.ContextName, "context", "override the current-context of the garden cluster kubeconfig")
	cmd.Flags().StringArrayVar(&o.Pattern, "pattern", nil, "define regex match patterns for this garden. This flag will overwrite the complete list. You can supply it multiple times to define multiple patterns for this garden")

	return cmd
}

// Run executes the command
func (o *SetGardenOptions) Run(f util.Factory) error {
	manager, err := f.Manager()
	if err != nil {
		return err
	}

	err = manager.Configuration().SetGarden(o.Identity, o.KubeconfigFile, o.ContextName, o.Pattern, f.GetConfigFile())
	if err != nil {
		return err
	}

	fmt.Fprintf(o.IOStreams.Out, "Successfully configured garden %q\n", o.Identity)

	return nil
}

// SetGardenOptions is a struct to support set command
type SetGardenOptions struct {
	base.Options

	// Identity identifies a garden cluster
	Identity string

	// KubeconfigFile is the path to the kubeconfig file of the Garden cluster that shall be added
	KubeconfigFile flag.StringFlag

	// ContextName Overrides the current-context of the garden cluster kubeconfig
	// +optional
	ContextName flag.StringFlag

	// Pattern is a list of regex patterns for targeting
	// +optional
	Pattern []string
}

// NewSetGardenOptions returns initialized SetGardenOptions
func NewSetGardenOptions(ioStreams util.IOStreams) *SetGardenOptions {
	return &SetGardenOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
}

// Complete adapts from the command line args to the data required.
func (o *SetGardenOptions) Complete(_ util.Factory, cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		o.Identity = strings.TrimSpace(args[0])
	}

	return nil
}

// Validate validates the provided options
func (o *SetGardenOptions) Validate() error {
	if o.Identity == "" {
		return errors.New("garden identity is required")
	}

	return nil
}
