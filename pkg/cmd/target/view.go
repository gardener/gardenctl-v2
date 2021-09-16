/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"encoding/json"
	"fmt"
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"gopkg.in/yaml.v3"

	"github.com/spf13/cobra"
)

// NewCmdView returns a new version command.
func NewCmdView(f util.Factory, o *ViewOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use: "view",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(f, cmd, args); err != nil {
				return fmt.Errorf("failed to complete command options: %w", err)
			}
			if err := o.Validate(); err != nil {
				return err
			}

			return runViewCommand(f, o)
		},
	}

	cmd.Flags().StringVarP(&o.Output, "output", "o", o.Output, "One of 'yaml' or 'json'.")

	return cmd
}

func runViewCommand(f util.Factory, opt *ViewOptions) error {
	m, err := f.Manager()
	if err != nil {
		return err
	}

	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return fmt.Errorf("failed to get current target: %v", err)
	}

	switch opt.Output {
	case "":
		fmt.Fprintf(opt.IOStreams.Out, "%v", currentTarget)

	case "yaml":
		marshalled, err := yaml.Marshal(&currentTarget)
		if err != nil {
			return err
		}

		fmt.Fprintln(opt.IOStreams.Out, string(marshalled))

	case "json":
		marshalled, err := json.MarshalIndent(&currentTarget, "", "  ")
		if err != nil {
			return err
		}

		fmt.Fprintln(opt.IOStreams.Out, string(marshalled))

	default:
		// There is a bug in the program if we hit this case.
		// However, we follow a policy of never panicking.
		return fmt.Errorf("options were not validated: --output=%q should have been rejected", opt.Output)
	}

	return nil
}

// ViewOptions is a struct to support version command
type ViewOptions struct {
	base.Options

	// Output defines the output format of the version information. Either 'yaml' or 'json'
	Output string
}

// NewViewOptions returns initialized ViewOptions
func NewViewOptions(ioStreams util.IOStreams) *ViewOptions {
	return &ViewOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
}

// Complete adapts from the command line args to the data required.
func (o *ViewOptions) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	return nil
}

// Validate validates the provided options
func (o *ViewOptions) Validate() error {
	if o.Output != "" && o.Output != "yaml" && o.Output != "json" {
		return fmt.Errorf(`--output must be either 'yaml' or 'json'`)
	}

	return nil
}
