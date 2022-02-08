/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
)

// NewCmdView returns a new target view command.
func NewCmdView(f util.Factory, o *ViewOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view",
		Short: "Print the current target",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Validate(); err != nil {
				return err
			}

			return runViewCommand(f, o)
		},
	}

	o.AddOutputFlags(cmd.Flags())

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

	if opt.Output == "" && currentTarget.IsEmpty() {
		_, err = fmt.Fprintf(opt.IOStreams.Out, "target is empty")
		return err
	}

	return opt.PrintObject(currentTarget)
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
