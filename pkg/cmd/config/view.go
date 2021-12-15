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

// NewCmdConfigView returns a new (config) view command.
func NewCmdConfigView(f util.Factory, o *ViewOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view",
		Short: "Print the gardenctl configuration",
		RunE:  base.WrapRunE(o, f),
	}

	o.AddFlags(cmd.Flags())

	return cmd
}

// Run executes the command
func (o *ViewOptions) Run(f util.Factory) error {
	m, err := f.Manager()
	if err != nil {
		return err
	}

	configuration := m.Configuration()

	if o.Output == "" {
		o.Output = "yaml"
	}

	return o.PrintObject(configuration)
}

// ViewOptions is a struct to support view command
type ViewOptions struct {
	base.Options
}

// NewViewOptions returns initialized ViewOptions
func NewViewOptions(ioStreams util.IOStreams) *ViewOptions {
	return &ViewOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
}
