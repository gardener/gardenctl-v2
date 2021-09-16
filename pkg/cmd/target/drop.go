/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"errors"
	"fmt"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"strings"

	"github.com/gardener/gardenctl-v2/internal/util"

	"github.com/spf13/cobra"
)

// NewCmdDrop returns a new (target) drop command.
func NewCmdDrop(f util.Factory, o *DropOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drop",
		Short: "Drop target, e.g. \"gardenctl target drop shoot\" to drop currently targeted shoot",
		ValidArgs: []string{
			string(TargetKindGarden),
			string(TargetKindProject),
			string(TargetKindSeed),
			string(TargetKindShoot),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(f, cmd, args); err != nil {
				return fmt.Errorf("failed to complete command options: %w", err)
			}
			if err := o.Validate(); err != nil {
				return err
			}

			return runCmdDrop(f, o)
		},
	}

	return cmd
}

func runCmdDrop(f util.Factory, o *DropOptions) error {
	manager, err := f.Manager()
	if err != nil {
		return err
	}

	var targetName string

	switch o.Kind {
	case TargetKindGarden:
		targetName, err = manager.DropTargetGarden()
	case TargetKindProject:
		targetName, err = manager.DropTargetProject()
	case TargetKindSeed:
		targetName, err = manager.DropTargetSeed()
	case TargetKindShoot:
		targetName, err = manager.DropTargetShoot()
	default:
		err = errors.New("invalid kind")
	}

	if err != nil {
		return err
	}

	fmt.Fprintf(o.IOStreams.Out, "Successfully dropped targeted %s %q\n", o.Kind, targetName)

	return nil
}

// DropOptions is a struct to support drop command
type DropOptions struct {
	base.Options

	// Kind is the target kind, for example "garden" or "seed"
	Kind TargetKind
}

// NewDropOptions returns initialized DropOptions
func NewDropOptions(ioStreams util.IOStreams) *DropOptions {
	return &DropOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
}

// Complete adapts from the command line args to the data required.
func (o *DropOptions) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		o.Kind = TargetKind(strings.TrimSpace(args[0]))
	}

	return nil
}

// Validate validates the provided options
func (o *DropOptions) Validate() error {
	if err := ValidateKind(o.Kind); err != nil {
		return err
	}

	return nil
}
