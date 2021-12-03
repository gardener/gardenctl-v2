/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"

	"github.com/spf13/cobra"
)

// NewCmdConfigDeleteGarden returns a new (config) set-garden command.
func NewCmdConfigDeleteGarden(f util.Factory, o *DeleteGardenOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete-garden",
		Short: "delete Garden from gardenctl configuration.",
		Long:  "Delete Garden from gardenctl configuration. E.g. \"gardenctl config delete-config my-garden to delete my-garden",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(f, cmd, args); err != nil {
				return fmt.Errorf("failed to complete command options: %w", err)
			}
			if err := o.Validate(); err != nil {
				return err
			}

			return runDeleteGardenCommand(f, o)
		},
	}

	return cmd
}

func runDeleteGardenCommand(f util.Factory, opt *DeleteGardenOptions) error {
	manager, err := f.Manager()
	if err != nil {
		return err
	}

	return manager.Configuration().DeleteGarden(opt.ClusterIdentity, f.GetConfigFile())
}

// DeleteGardenOptions is a struct to support view command
type DeleteGardenOptions struct {
	base.Options

	// ClusterIdentity identifies a garden cluster
	ClusterIdentity string
}

// NewDeleteGardenOptions returns initialized DeleteGardenOptions
func NewDeleteGardenOptions(ioStreams util.IOStreams) *DeleteGardenOptions {
	return &DeleteGardenOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
}

// Complete adapts from the command line args to the data required.
func (o *DeleteGardenOptions) Complete(_ util.Factory, cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		o.ClusterIdentity = strings.TrimSpace(args[0])
	}

	return nil
}

// Validate validates the provided options
func (o *DeleteGardenOptions) Validate() error {
	if o.ClusterIdentity == "" {
		return errors.New("garden cluster identity is required")
	}

	return nil
}
