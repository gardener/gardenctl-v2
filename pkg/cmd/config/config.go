/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"

	"github.com/spf13/cobra"
)

// NewCmdConfig returns a new config command.
func NewCmdConfig(f util.Factory, o *ConfigOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "modify gardenctl configuration file using subcommands",
		Long: `Modify gardenctl files using subcommands like \"gardenctl config set-garden my-garden\"

The loading order follows these rules:
1. If the --config flag is set, then only that file is loaded.
2. If path provided with the env variable GCTL_HOME gardenctl will search for config file there. Otherwise, the home directory will default to ${HOME}/.garden/.\
3. If config filename set with env variable GCTL_CONFIG_NAME it will be used. Otherwise, the config filename will default to gardenctl-v2.yaml.`,
	}

	ioStreams := util.NewIOStreams()

	cmd.AddCommand(NewCmdConfigView(f, NewViewOptions(ioStreams)))
	cmd.AddCommand(NewCmdConfigSetGarden(f, NewSetGardenOptions(ioStreams)))
	cmd.AddCommand(NewCmdConfigDeleteGarden(f, NewDeleteGardenOptions(ioStreams)))

	return cmd
}

// ConfigOptions is a struct to support config command
// nolint
type ConfigOptions struct {
	base.Options
}

// NewConfigOptions returns initialized NewConfigOptions
// nolint
func NewConfigOptions(ioStreams util.IOStreams) *ConfigOptions {
	return &ConfigOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
}

func validGardenArgsFunction(f util.Factory, args []string) ([]string, error) {
	manager, err := f.Manager()
	if err != nil {
		return nil, err
	}

	var identities []string

	if len(args) == 0 {
		for _, g := range manager.Configuration().Gardens {
			identities = append(identities, g.Identity)
		}

		return identities, nil
	}

	return nil, nil
}
