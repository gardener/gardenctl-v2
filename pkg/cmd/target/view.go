/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/flags"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

// NewCmdView returns a new target view command.
func NewCmdView(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := &TargetViewOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
	cmd := &cobra.Command{
		Use:   "view",
		Short: "Print the current target",
		RunE:  base.WrapRunE(o, f),
	}

	o.AddFlags(cmd.Flags())
	o.RegisterCompletionsForOutputFlag(cmd)

	f.TargetFlags().AddFlags(cmd.Flags())
	flags.RegisterCompletionFuncsForTargetFlags(cmd, f, ioStreams, cmd.Flags())

	return cmd
}

// TargetViewOptions is a struct to support view command.
type TargetViewOptions struct {
	base.Options
	Target target.Target
}

// Complete adapts from the command line args to the data required.
func (o *TargetViewOptions) Complete(f util.Factory, _ *cobra.Command, _ []string) error {
	m, err := f.Manager()
	if err != nil {
		return err
	}

	o.Target, err = m.CurrentTarget()
	if err != nil {
		return err
	}

	// Use 'yaml' as the default output format
	if o.Output == "" {
		o.Output = "yaml"
	}

	return nil
}

// Run executes the command.
func (o *TargetViewOptions) Run(_ util.Factory) error {
	return o.PrintObject(o.Target)
}
