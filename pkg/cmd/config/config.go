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
		Short: "Modify gardenctl configuration file using subcommands",
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

	var gNames []string

	if len(args) == 0 {
		for _, g := range manager.Configuration().AllGardens() {
			gNames = append(gNames, g.Identity)
		}

		return gNames, nil
	}

	return nil, nil
}
