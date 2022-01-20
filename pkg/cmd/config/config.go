/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"errors"
	"fmt"

	"github.com/gardener/gardenctl-v2/pkg/cmd/base"

	"github.com/gardener/gardenctl-v2/internal/util"

	"github.com/spf13/cobra"
)

// NewCmdConfig returns a new config command.
func NewCmdConfig(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "modify gardenctl configuration file using subcommands",
		Long: `Modify gardenctl files using subcommands like "gardenctl config set-garden my-garden"

The loading order follows these rules:
1. If the --config flag is set, then only that file is loaded.
2. If $GCTL_HOME environment variable is set, then it is used as primary search path for the config file. The secondary search path of the home directory is ${HOME}/.garden/.
3. If $GCTL_CONFIG_NAME environment variable is set, then it is used as config filename. Otherwise, the config filename will default to gardenctl-v2. The config name must not include the file extension`,
	}

	cmd.AddCommand(NewCmdConfigView(f, ioStreams))
	cmd.AddCommand(NewCmdConfigSetGarden(f, ioStreams))
	cmd.AddCommand(NewCmdConfigDeleteGarden(f, ioStreams))

	return cmd
}

func validGardenArgsFunction(f util.Factory, args []string) ([]string, error) {
	if len(args) > 0 {
		return nil, nil
	}

	manager, err := f.Manager()
	if err != nil {
		return nil, fmt.Errorf("failed to get target manager: %w", err)
	}

	config := manager.Configuration()
	if config == nil {
		return nil, errors.New("failed to get configuration")
	}

	return config.GardenNames(), nil
}

// NewCmdConfigView returns a new (config) view command.
func NewCmdConfigView(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	use := "view"
	o := &options{
		Options: base.Options{
			IOStreams: ioStreams,
		},
		Command: use,
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: "Print the gardenctl configuration",
		RunE:  base.WrapRunE(o, f),
	}

	o.AddFlags(cmd.Flags())

	return cmd
}

// NewCmdConfigSetGarden returns a new (config) set-garden command.
func NewCmdConfigSetGarden(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	use := "set-garden"
	o := &options{
		Options: base.Options{
			IOStreams: ioStreams,
		},
		Command: use,
	}
	cmd := &cobra.Command{
		Use:   use,
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

	o.AddFlags(cmd.Flags())

	return cmd
}

// NewCmdConfigDeleteGarden returns a new (config) delete-garden command.
func NewCmdConfigDeleteGarden(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	use := "delete-garden"
	o := &options{
		Options: base.Options{
			IOStreams: ioStreams,
		},
		Command: use,
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: "delete Garden from gardenctl configuration",
		Long:  "Delete Garden from gardenctl configuration. E.g. \"gardenctl config delete-garden my-garden\" to delete my-garden",
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

	return cmd
}
