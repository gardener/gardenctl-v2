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
		Short: "modify or add Garden to gardenctl configuration.",
		Long:  "Modify or add Garden to gardenctl configuration. E.g. \"gardenctl config set-config my-garden --kubeconfig ~/.kube/kubeconfig.yaml\" to configure or add a garden with identity 'my-garden'.",
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			suggestions, err := validGardenArgsFunction(f, args)
			if err != nil {
				fmt.Fprintln(o.IOStreams.ErrOut, err.Error())
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			return util.FilterStringsByPrefix(toComplete, suggestions), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(f, cmd, args); err != nil {
				return fmt.Errorf("failed to complete command options: %w", err)
			}
			if err := o.Validate(); err != nil {
				return err
			}

			return runSetGardenCommand(f, o)
		},
	}

	cmd.Flags().Var(&o.KubeconfigFile, "kubeconfig", "path to kubeconfig file for this Garden cluster. If used without --context, current-context of kubeconfig will be set as context")
	cmd.Flags().Var(&o.ContextName, "context", "use specific context of kubeconfig")
	cmd.Flags().Var(&o.Short, "short", "use alternative name for targeting this garden")
	cmd.Flags().StringArrayVar(&o.Patterns, "patterns", nil, "define regex match patterns for this garden. This flag will overwrite the complete list")

	return cmd
}

func runSetGardenCommand(f util.Factory, opt *SetGardenOptions) error {
	manager, err := f.Manager()
	if err != nil {
		return err
	}

	return manager.Configuration().SetGarden(opt.Identity, opt.KubeconfigFile, opt.ContextName, opt.Short, opt.Patterns, f.GetConfigFile())
}

// SetGardenOptions is a struct to support view command
type SetGardenOptions struct {
	base.Options

	// Identity identifies a garden cluster
	Identity string

	// KubeconfigFile is the path to the kubeconfig file of the Garden cluster that shall be added
	KubeconfigFile flag.StringFlag

	// Context to use for kubeconfig
	ContextName flag.StringFlag

	// Short is an alternative name to identify this cluster
	Short flag.StringFlag

	// Patterns is a list of regex patterns for targeting
	Patterns []string
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
