/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"

	"github.com/spf13/cobra"
)

// NewCmdUnset returns a new (target) unset command.
func NewCmdUnset(f util.Factory, o *UnsetOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unset",
		Short: "Unset target, e.g. \"gardenctl target unset shoot\" to unset currently targeted shoot",
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

			return runCmdUnset(f, o)
		},
	}

	return cmd
}

func runCmdUnset(f util.Factory, o *UnsetOptions) error {
	manager, err := f.Manager()
	if err != nil {
		return err
	}

	var targetName string

	switch o.Kind {
	case TargetKindGarden:
		targetName, err = manager.UnsetTargetGarden()
	case TargetKindProject:
		targetName, err = manager.UnsetTargetProject()
	case TargetKindSeed:
		targetName, err = manager.UnsetTargetSeed()
	case TargetKindShoot:
		targetName, err = manager.UnsetTargetShoot()
	default:
		err = errors.New("invalid kind")
	}

	if err != nil {
		return err
	}

	fmt.Fprintf(o.IOStreams.Out, "Successfully unset targeted %s %q\n", o.Kind, targetName)

	return nil
}

// UnsetOptions is a struct to support unset command
type UnsetOptions struct {
	base.Options

	// Kind is the target kind, for example "garden" or "seed"
	Kind TargetKind
}

// NewUnsetOptions returns initialized UnsetOptions
func NewUnsetOptions(ioStreams util.IOStreams) *UnsetOptions {
	return &UnsetOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
}

// Complete adapts from the command line args to the data required.
func (o *UnsetOptions) Complete(_ util.Factory, cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		o.Kind = TargetKind(strings.TrimSpace(args[0]))
	}

	return nil
}

// Validate validates the provided options
func (o *UnsetOptions) Validate() error {
	if err := ValidateKind(o.Kind); err != nil {
		return err
	}

	return nil
}
