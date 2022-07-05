/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/ac"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/flags"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

// NewCmdTarget returns a new target command.
func NewCmdTarget(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := NewTargetOptions(ioStreams)
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
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Use == "target" {
				s, err := HistoryParse(f, cmd)
				if err != nil {
					return err
				}
				return HistoryWrite(historyPath(f), s)
			}
			return nil
		},
	}

	cmd.AddCommand(NewCmdTargetGarden(f, ioStreams))
	cmd.AddCommand(NewCmdTargetProject(f, ioStreams))
	cmd.AddCommand(NewCmdTargetShoot(f, ioStreams))
	cmd.AddCommand(NewCmdTargetSeed(f, ioStreams))
	cmd.AddCommand(NewCmdTargetControlPlane(f, ioStreams))

	cmd.AddCommand(NewCmdUnset(f, ioStreams))
	cmd.AddCommand(NewCmdView(f, ioStreams))
	cmd.AddCommand(NewCmdHistory(f, o.Options))
	o.AddFlags(cmd.Flags())

	manager, err := f.Manager()
	utilruntime.Must(err)
	manager.TargetFlags().AddFlags(cmd.PersistentFlags())
	flags.RegisterCompletionFuncsForTargetFlags(cmd, f, ioStreams, cmd.PersistentFlags())

	return cmd
}

// TargetKind is representing the type of things that can be targeted
// by this cobra command. While this may sound stuttery, the alternative
// of just calling it "Kind" is even worse, hence the nolint.
//
//nolint:revive
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
		result, err = manager.GardenNames()
	case TargetKindProject:
		result, err = manager.ProjectNames(ctx)
	case TargetKindSeed:
		result, err = manager.SeedNames(ctx)
	case TargetKindShoot:
		result, err = manager.ShootNames(ctx)
	}

	return result, err
}

// TargetOptions is a struct to support target command
//
//nolint:revive
type TargetOptions struct {
	base.Options

	// Kind is the target kind, for example "garden" or "seed"
	Kind TargetKind
	// TargetName is the object name of the targeted kind
	TargetName string
}

// NewTargetOptions returns initialized TargetOptions.
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
		switch {
		case tf.ControlPlane():
			o.Kind = TargetKindControlPlane
		case tf.ShootName() != "":
			o.Kind = TargetKindShoot
		case tf.ProjectName() != "":
			o.Kind = TargetKindProject
		case tf.SeedName() != "":
			o.Kind = TargetKindSeed
		case tf.GardenName() != "":
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

// Validate validates the provided options.
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

// Run executes the command.
func (o *TargetOptions) Run(f util.Factory) error {
	manager, err := f.Manager()
	if err != nil {
		return err
	}

	askForConfirmation := manager.TargetFlags().ShootName() != "" || o.Kind == TargetKindShoot
	handler := ac.NewAccessRestrictionHandler(o.IOStreams.In, o.IOStreams.Out, askForConfirmation)
	ctx := ac.WithAccessRestrictionHandler(f.Context(), handler)

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
		if errors.Is(err, target.ErrAborted) {
			return nil
		}

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

	if manager.Configuration().SymlinkTargetKubeconfig() {
		if os.Getenv("KUBECONFIG") != filepath.Join(manager.SessionDir(), "kubeconfig.yaml") {
			fmt.Fprintf(o.IOStreams.Out, "%s The KUBECONFIG environment variable does not point to the current target of gardenctl. Run `gardenctl kubectl-env --help` on how to configure the KUBECONFIG environment variable accordingly\n", color.YellowString("WARN"))
		}
	}

	return nil
}
