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
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Validate(); err != nil {
				return err
			}

			return runViewCommand(f, o)
		},
	}

	o.AddFlags(cmd.Flags())

	return cmd
}

func runViewCommand(f util.Factory, opt *ViewOptions) error {
	m, err := f.Manager()
	if err != nil {
		return err
	}

	configuration := m.Configuration()

	if opt.Output == "" {
		opt.Output = "yaml"
	}

	return opt.PrintObject(configuration)
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
