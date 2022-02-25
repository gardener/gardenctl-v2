/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/config"
)

// NewCmdConfigView returns a new (config) view command.
func NewCmdConfigView(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := &viewOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
	cmd := &cobra.Command{
		Use:   "view",
		Short: "Print the gardenctl configuration",
		Example: `# view current configuration
gardenctl config view`,
		RunE: base.WrapRunE(o, f),
	}

	o.AddOutputFlag(cmd.Flags())

	return cmd
}

type viewOptions struct {
	base.Options
	// Configuration is the gardenctl configuration
	Configuration *config.Config
}

// Complete adapts from the command line args to the data required.
func (o *viewOptions) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	config, err := getConfiguration(f)
	if err != nil {
		return err
	}

	o.Configuration = config

	if o.Output == "" {
		o.Output = "yaml"
	}

	return nil
}

// Run executes the command
func (o *viewOptions) Run(_ util.Factory) error {
	return o.PrintObject(o.Configuration)
}
