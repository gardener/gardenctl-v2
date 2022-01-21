/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/config"
)

// NewCmdConfigDeleteGarden returns a new (config) delete-garden command.
func NewCmdConfigDeleteGarden(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := &deleteGardenOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
	cmd := &cobra.Command{
		Use:   "delete-garden",
		Short: "delete Garden from gardenctl configuration",
		Long:  "Delete Garden from gardenctl configuration. E.g. \"gardenctl config delete-garden my-garden\" to delete my-garden",
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			suggestions, err := validGardenArgsFunction(f, args)
			if err != nil {
				fmt.Fprintln(o.IOStreams.ErrOut, err.Error())
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			return util.FilterStringsByPrefix(toComplete, suggestions), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: base.WrapRunE(o, f),
	}

	return cmd
}

type deleteGardenOptions struct {
	base.Options
	// Configuration is the gardenctl configuration
	Configuration *config.Config
	// Name is a unique name of this Garden that can be used to target this Garden
	Name string
}

// Complete adapts from the command line args to the data required.
func (o *deleteGardenOptions) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	manager, err := f.Manager()
	if err != nil {
		return fmt.Errorf("failed to get target manager: %w", err)
	}

	config := manager.Configuration()
	if config == nil {
		return errors.New("failed to get configuration")
	}

	o.Configuration = config

	if len(args) > 0 {
		o.Name = strings.TrimSpace(args[0])
	}

	return nil
}

// Validate validates the provided options
func (o *deleteGardenOptions) Validate() error {
	if o.Name == "" {
		return errors.New("garden identity is required")
	}

	return nil
}

// Run executes the command
func (o *deleteGardenOptions) Run(_ util.Factory) error {
	err := o.Configuration.DeleteGarden(o.Name)
	if err != nil {
		return err
	}

	err = o.Configuration.Save()
	if err != nil {
		return fmt.Errorf("failed to delete garden from configuration: %w", err)
	}

	fmt.Fprintf(o.IOStreams.Out, "Successfully deleted garden %q\n", o.Name)

	return nil
}
