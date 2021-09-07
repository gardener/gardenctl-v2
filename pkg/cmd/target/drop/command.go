/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package drop

import (
	"errors"
	"fmt"
	"github.com/gardener/gardenctl-v2/pkg/target"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/spf13/cobra"
)

// NewCommand returns a new target command.
func NewCommand(f util.Factory, o *Options, targetProvider *target.DynamicTargetProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drop",
		Short: "Drop target, e.g. \"gardenctl target drop shoot\" to drop currently targeted shoot",
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			suggestions, err := validArgsFunction(f, o, args, toComplete)
			if err != nil {
				fmt.Fprintln(o.IOStreams.ErrOut, err.Error())
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			return util.FilterStringsByPrefix(toComplete, suggestions), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(f, cmd, args, targetProvider); err != nil {
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
