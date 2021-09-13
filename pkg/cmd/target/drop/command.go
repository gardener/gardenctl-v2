/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package drop

import (
	"errors"
	"fmt"

	"github.com/gardener/gardenctl-v2/internal/util"
	commonTarget "github.com/gardener/gardenctl-v2/pkg/cmd/common/target"
	"github.com/gardener/gardenctl-v2/pkg/target"

	"github.com/spf13/cobra"
)

// NewCommand returns a new (target) drop command.
func NewCommand(f util.Factory, o *Options, targetProvider *target.DynamicTargetProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drop",
		Short: "Drop target, e.g. \"gardenctl target drop shoot\" to drop currently targeted shoot",
		ValidArgs: []string{
			string(commonTarget.TargetKindGarden),
			string(commonTarget.TargetKindProject),
			string(commonTarget.TargetKindSeed),
			string(commonTarget.TargetKindShoot),
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
	case commonTarget.TargetKindGarden:
		targetName, err = manager.DropTargetGarden()
	case commonTarget.TargetKindProject:
		targetName, err = manager.DropTargetProject()
	case commonTarget.TargetKindSeed:
		targetName, err = manager.DropTargetSeed()
	case commonTarget.TargetKindShoot:
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
