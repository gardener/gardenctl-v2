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
		Short: "delete the specified Garden from the gardenctl configuration",
		Example: `# delete my-garden
gardenctl config delete-garden my-garden`,
		ValidArgsFunction: validGardenArgsFunctionWrapper(f, ioStreams),
		RunE:              base.WrapRunE(o, f),
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
	config, err := getConfiguration(f)
	if err != nil {
		return err
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
	i, ok := o.Configuration.IndexOfGarden(o.Name)
	if !ok {
		return fmt.Errorf("garden %q is not defined in gardenctl configuration", o.Name)
	}

	o.Configuration.Gardens = append(o.Configuration.Gardens[:i], o.Configuration.Gardens[i+1:]...)

	err := o.Configuration.Save()
	if err != nil {
		return fmt.Errorf("failed to delete garden from configuration: %w", err)
	}

	fmt.Fprintf(o.IOStreams.Out, "Successfully deleted garden %q\n", o.Name)

	return nil
}
