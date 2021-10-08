/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"fmt"
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/spf13/cobra"
)

// NewCmdView returns a new version command.
func NewCmdView(f util.Factory, o *ViewOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view",
		Short: "Print the current target",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(f, cmd, args); err != nil {
				return fmt.Errorf("failed to complete command options: %w", err)
			}
			if err := o.Validate(); err != nil {
				return err
			}

			return runViewCommand(f, o)
		},
	}

	o.AddOutputFlags(cmd)

	return cmd
}

func runViewCommand(f util.Factory, opt *ViewOptions) error {
	m, err := f.Manager()
	if err != nil {
		return err
	}

	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return fmt.Errorf("failed to get current target: %v", err)
	}

	if currentTarget.IsEmpty() {
		fmt.Fprintf(opt.IOStreams.Out, "Target is empty. Check gardenctl target --help on how to use the target command")
		return nil
	}
	return opt.PrintObject(currentTarget)
}

// ViewOptions is a struct to support version command
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
func (o *ViewOptions) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	return nil
}
