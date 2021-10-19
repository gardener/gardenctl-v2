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

// NewCmdTarget returns a new target command.
func NewCmdTarget(f util.Factory, o *TargetOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "target",
		Short: "Set scope for next operations, e.g. \"gardenctl target garden garden_name\" to target garden with name of garden_name",
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			suggestions, err := validTargetArgsFunction(f, o, args, toComplete)
			if err != nil {
				fmt.Fprintln(o.IOStreams.ErrOut, err.Error())
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			return util.FilterStringsByPrefix(toComplete, suggestions), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(f, cmd, args); err != nil {
				return fmt.Errorf("failed to complete command options: %w", err)
			}

			if err := o.Validate(); err != nil {
				return err
			}

			return runCmdTarget(f, o)
		},
	}

	ioStreams := util.NewIOStreams()

	cmd.AddCommand(NewCmdUnset(f, NewUnsetOptions(ioStreams)))
	cmd.AddCommand(NewCmdView(f, NewViewOptions(ioStreams)))

	return cmd
}

func runCmdTarget(f util.Factory, o *TargetOptions) error {
	manager, err := f.Manager()
	if err != nil {
		return err
	}

	ctx := f.Context()

	switch o.Kind {
	case TargetKindGarden:
		err = manager.TargetGarden(o.TargetName)
	case TargetKindProject:
		err = manager.TargetProject(ctx, o.TargetName)
	case TargetKindSeed:
		err = manager.TargetSeed(ctx, o.TargetName)
	case TargetKindShoot:
		err = manager.TargetShoot(ctx, o.TargetName)
	default:
		err = manager.TargetMatchPattern(ctx, o.TargetName)
	}

	if err != nil {
		return err
	}

	currentTarget, err := manager.CurrentTarget()
	if err != nil {
		return fmt.Errorf("failed to get current target: %v", err)
	}

	fmt.Fprintf(o.IOStreams.Out, "Successfully targeted %s %q\n", o.Kind, o.TargetName)
	fmt.Fprintf(o.IOStreams.Out, "New target: %s\n", currentTarget)

	return nil
}

// TargetKind is representing the type of things that can be targeted
// by this cobra command. While this may sound stuttery, the alternative
// of just calling it "Kind" is even worse, hence the nolint.
// nolint
type TargetKind string

const (
	TargetKindGarden  TargetKind = "garden"
	TargetKindProject TargetKind = "project"
	TargetKindSeed    TargetKind = "seed"
	TargetKindShoot   TargetKind = "shoot"
	TargetKindPattern TargetKind = "pattern"
)

var (
	AllTargetKinds = []TargetKind{TargetKindGarden, TargetKindProject, TargetKindSeed, TargetKindShoot, TargetKindPattern}
)

func ValidateKind(kind TargetKind) error {
	for _, k := range AllTargetKinds {
		if k == kind {
			return nil
		}
	}

	return fmt.Errorf("invalid target kind given, must be one of %v", AllTargetKinds)
}

func validTargetArgsFunction(f util.Factory, o *TargetOptions, args []string, toComplete string) ([]string, error) {
	if len(args) == 0 {
		return []string{
			string(TargetKindGarden),
			string(TargetKindProject),
			string(TargetKindSeed),
			string(TargetKindShoot),
			string(TargetKindPattern),
		}, nil
	}

	kind := TargetKind(strings.TrimSpace(args[0]))
	if err := ValidateKind(kind); err != nil {
		return nil, err
	}

	manager, err := f.Manager()
	if err != nil {
		return nil, err
	}

	// NB: this uses the DynamicTargetProvider from the root cmd and
	// is therefore aware of flags like --garden; the goal here is to
	// allow the user to type "gardenctl target --garden [tab][select] --project [tab][select] shoot [tab][select]"
	currentTarget, err := manager.CurrentTarget()
	if err != nil {
		return nil, err
	}

	ctx := f.Context()

	var result []string

	switch kind {
	case TargetKindGarden:
		result, err = util.GardenNames(manager)
	case TargetKindProject:
		result, err = util.ProjectNamesForTarget(ctx, manager, currentTarget)
	case TargetKindSeed:
		result, err = util.SeedNamesForTarget(ctx, manager, currentTarget)
	case TargetKindShoot:
		result, err = util.ShootNamesForTarget(ctx, manager, currentTarget)
	}

	return result, err
}

// TargetOptions is a struct to support target command
// nolint
type TargetOptions struct {
	base.Options

	// Kind is the target kind, for example "garden" or "seed"
	Kind TargetKind
	// TargetName is the object name of the targeted kind
	TargetName string
}

// NewTargetOptions returns initialized TargetOptions
func NewTargetOptions(ioStreams util.IOStreams) *TargetOptions {
	return &TargetOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
}

// Complete adapts from the command line args to the data required.
func (o *TargetOptions) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		o.Kind = TargetKind(strings.TrimSpace(args[0]))
	}

	kindValidationErr := ValidateKind(o.Kind)
	if len(args) == 1 && kindValidationErr != nil {
		// no target kind provided - try to match target with match patterns
		o.TargetName = strings.TrimSpace(args[0])
		o.Kind = TargetKindPattern
	} else {
		if len(args) > 1 {
			o.TargetName = strings.TrimSpace(args[1])
		}
	}

	manager, err := f.Manager()
	if err != nil {
		return err
	}

	tf := manager.TargetFlags()
	if o.Kind == "" {
		if tf.ShootName() != "" {
			o.Kind = TargetKindShoot
		} else if tf.ProjectName() != "" {
			o.Kind = TargetKindProject
		} else if tf.SeedName() != "" {
			o.Kind = TargetKindSeed
		} else if tf.GardenName() != "" {
			o.Kind = TargetKindGarden
		}
	}

	if o.TargetName == "" {
		switch o.Kind {
		case TargetKindGarden:
			o.TargetName = tf.GardenName()
		case TargetKindProject:
			o.TargetName = tf.ProjectName()
		case TargetKindSeed:
			o.TargetName = tf.SeedName()
		case TargetKindShoot:
			o.TargetName = tf.ShootName()
		}
	}

	return nil
}

// Validate validates the provided options
func (o *TargetOptions) Validate() error {
	// reject flag/arg-less invocations
	if o.Kind == "" || o.TargetName == "" {
		return errors.New("no target specified")
	}

	if err := ValidateKind(o.Kind); err != nil {
		return err
	}

	return nil
}
