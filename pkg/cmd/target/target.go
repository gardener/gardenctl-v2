/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
)

// NewCmdTarget returns a new target command.
func NewCmdTarget(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := &TargetOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
	cmd := &cobra.Command{
		Use:   "target",
		Short: "Set scope for next operations, using subcommands or pattern",
		Example: `# target project "my-project" of garden "my-garden"
gardenctl target --garden my-garden --project my-project

# target shoot "my-shoot" of currently selected project
gardenctl target shoot my-shoot

# Target shoot control-plane using values that match a pattern defined for a specific garden
gardenctl target value/that/matches/pattern --control-plane`,
		RunE: base.WrapRunE(o, f),
	}

	cmd.AddCommand(NewCmdTargetGarden(f, ioStreams))
	cmd.AddCommand(NewCmdTargetProject(f, ioStreams))
	cmd.AddCommand(NewCmdTargetShoot(f, ioStreams))
	cmd.AddCommand(NewCmdTargetSeed(f, ioStreams))
	cmd.AddCommand(NewCmdTargetControlPlane(f, ioStreams))

	cmd.AddCommand(NewCmdUnset(f, NewUnsetOptions(ioStreams)))
	cmd.AddCommand(NewCmdView(f, NewViewOptions(ioStreams)))

	o.AddFlags(cmd.Flags())

	return cmd
}

// TargetKind is representing the type of things that can be targeted
// by this cobra command. While this may sound stuttery, the alternative
// of just calling it "Kind" is even worse, hence the nolint.
// nolint
type TargetKind string

const (
	TargetKindGarden       TargetKind = "garden"
	TargetKindProject      TargetKind = "project"
	TargetKindSeed         TargetKind = "seed"
	TargetKindShoot        TargetKind = "shoot"
	TargetKindPattern      TargetKind = "pattern"
	TargetKindControlPlane TargetKind = "control-plane"
)

type cobraValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

func validTargetFunctionWrapper(f util.Factory, ioStreams util.IOStreams, kind TargetKind) cobraValidArgsFunction {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		suggestions, err := validTargetArgsFunction(f, kind)
		if err != nil {
			fmt.Fprintln(ioStreams.ErrOut, err.Error())
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		return util.FilterStringsByPrefix(toComplete, suggestions), cobra.ShellCompDirectiveNoFileComp
	}
}

func validTargetArgsFunction(f util.Factory, kind TargetKind) ([]string, error) {
	manager, err := f.Manager()
	if err != nil {
		return nil, err
	}

	ctx := f.Context()

	var result []string

	switch kind {
	case TargetKindGarden:
		result, err = util.GardenNames(manager)
	case TargetKindProject:
		result, err = util.ProjectNamesForTarget(ctx, manager)
	case TargetKindSeed:
		result, err = util.SeedNamesForTarget(ctx, manager)
	case TargetKindShoot:
		result, err = util.ShootNamesForTarget(ctx, manager)
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
		if o.Kind == "" {
			o.Kind = TargetKindPattern
		}

		o.TargetName = strings.TrimSpace(args[0])
	}

	manager, err := f.Manager()
	if err != nil {
		return err
	}

	tf := manager.TargetFlags()
	if o.Kind == "" {
		if tf.ControlPlane() {
			o.Kind = TargetKindControlPlane
		} else if tf.ShootName() != "" {
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
	switch o.Kind {
	case TargetKindControlPlane:
		// valid
	default:
		if o.TargetName == "" {
			return fmt.Errorf("target kind %q requires a name argument", o.Kind)
		}
	}

	return nil
}

// Run executes the command
func (o *TargetOptions) Run(f util.Factory) error {
	manager, err := f.Manager()
	if err != nil {
		return err
	}

	ctx := f.Context()

	switch o.Kind {
	case TargetKindGarden:
		err = manager.TargetGarden(ctx, o.TargetName)
	case TargetKindProject:
		err = manager.TargetProject(ctx, o.TargetName)
	case TargetKindSeed:
		err = manager.TargetSeed(ctx, o.TargetName)
	case TargetKindShoot:
		err = manager.TargetShoot(ctx, o.TargetName)
	case TargetKindPattern:
		err = manager.TargetMatchPattern(ctx, o.TargetName)
	case TargetKindControlPlane:
		err = manager.TargetControlPlane(ctx)
	}

	if err != nil {
		return err
	}

	currentTarget, err := manager.CurrentTarget()
	if err != nil {
		return fmt.Errorf("failed to get current target: %w", err)
	}

	if o.Output == "" {
		if o.Kind == TargetKindControlPlane {
			fmt.Fprintf(o.IOStreams.Out, "Successfully targeted control plane of shoot %q\n", currentTarget.ShootName())
		} else if o.Kind != "" {
			fmt.Fprintf(o.IOStreams.Out, "Successfully targeted %s %q\n", o.Kind, o.TargetName)
		}
	}

	return nil
}
