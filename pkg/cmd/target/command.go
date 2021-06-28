/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"errors"
	"fmt"

	"github.com/gardener/gardenctl-v2/internal/util"

	"github.com/spf13/cobra"
)

// NewCommand returns a new target command.
func NewCommand(f util.Factory, o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "target",
		Short: "Set scope for next operations, e.g. \"gardenctl target garden garden_name\" to target garden with name of garden_name",
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			suggestions, err := validArgsFunction(f, o, args, toComplete)
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

			return runCommand(f, o)
		},
	}

	return cmd
}

func runCommand(f util.Factory, o *Options) error {
	manager, err := f.Manager()
	if err != nil {
		return err
	}

	// in case the user didn't specify all arguments, we fake them
	// by looking at the CLI flags instead. This allows the user to do a
	// "gardenctl target --shoot foo" and still have it working
	currentTarget, err := manager.CurrentTarget()
	if err != nil {
		return err
	}

	if o.Kind == "" {
		if currentTarget.ShootName() != "" {
			o.Kind = TargetKindShoot
		} else if currentTarget.ProjectName() != "" {
			o.Kind = TargetKindProject
		} else if currentTarget.SeedName() != "" {
			o.Kind = TargetKindSeed
		} else if currentTarget.GardenName() != "" {
			o.Kind = TargetKindGarden
		}
	}

	if o.TargetName == "" {
		switch o.Kind {
		case TargetKindGarden:
			o.TargetName = currentTarget.GardenName()
		case TargetKindProject:
			o.TargetName = currentTarget.ProjectName()
		case TargetKindSeed:
			o.TargetName = currentTarget.SeedName()
		case TargetKindShoot:
			o.TargetName = currentTarget.ShootName()
		}
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
		err = errors.New("invalid kind")
	}

	if err != nil {
		return err
	}

	fmt.Fprintf(o.IOStreams.Out, "Successfully targeted %s %q.\n", o.Kind, o.TargetName)

	return nil
}
