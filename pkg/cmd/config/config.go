/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/config"
)

// NewCmdConfig returns a new config command.
func NewCmdConfig(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Modify gardenctl configuration file using subcommands",
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

type cobraValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

func validGardenArgsFunctionWrapper(f util.Factory, ioStreams util.IOStreams) cobraValidArgsFunction {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		config, err := getConfiguration(f)
		if err != nil {
			fmt.Fprintln(ioStreams.ErrOut, err.Error())
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		return util.FilterStringsByPrefix(toComplete, config.GardenNames()), cobra.ShellCompDirectiveNoFileComp
	}
}

func getConfiguration(f util.Factory) (*config.Config, error) {
	manager, err := f.Manager()
	if err != nil {
		return nil, fmt.Errorf("failed to get target manager: %w", err)
	}

	config := manager.Configuration()
	if config == nil {
		return nil, errors.New("failed to get configuration")
	}

	return config, nil
}
