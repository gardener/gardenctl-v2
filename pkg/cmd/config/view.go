/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"errors"
	"fmt"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"

	"github.com/spf13/cobra"
)

// NewCmdConfigView returns a new (config) view command.
func NewCmdConfigView(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := NewViewOptions(ioStreams)
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
		return fmt.Errorf("failed to get target manager: %w", err)
	}

	config := m.Configuration()
	if config == nil {
		return errors.New("could not get configuration")
	}

	return o.PrintObject(config)
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

// Complete adapts from the command line args to the data required.
func (o *ViewOptions) Complete(_ util.Factory, cmd *cobra.Command, args []string) error {
	if o.Output == "" {
		o.Output = "yaml"
	}

	return nil
}
