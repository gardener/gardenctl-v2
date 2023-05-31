/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package base

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/yaml"

	"github.com/gardener/gardenctl-v2/internal/util"
)

//go:generate mockgen -destination=./mocks/mock_options.go -package=mocks github.com/gardener/gardenctl-v2/pkg/cmd/base Runnable

// Runnable is the base interface for command options.
type Runnable interface {
	// Complete adapts from the command line args to the data required.
	Complete(util.Factory, *cobra.Command, []string) error
	// Validate validates the provided options.
	Validate() error
	// Run does the actual work of the command.
	Run(util.Factory) error
}

// Options contains all settings that are used across all commands in gardenctl.
type Options struct {
	// IOStreams provides the standard names for iostreams
	IOStreams util.IOStreams

	// Output defines the output format of the version information. Either 'yaml' or 'json'
	Output string
}

var _ Runnable = &Options{}

// WrapRunE creates a cobra RunE function that has access to the factory.
func WrapRunE(o Runnable, f util.Factory) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := o.Complete(f, cmd, args); err != nil {
			return fmt.Errorf("failed to complete command options: %w", err)
		}

		if err := o.Validate(); err != nil {
			return err
		}

		return o.Run(f)
	}
}

// NewOptions returns initialized Options.
func NewOptions(ioStreams util.IOStreams) *Options {
	return &Options{
		IOStreams: ioStreams,
	}
}

// AddFlags adds flags to adjust the output to a cobra command.
func (o *Options) AddFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&o.Output, "output", "o", o.Output, "One of 'yaml' or 'json'.")
}

// RegisterCompletionsForOutputFlag adds output flag completion to the command.
func (o *Options) RegisterCompletionsForOutputFlag(cmd *cobra.Command) {
	utilruntime.Must(cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return o.AllowedOutputFormats(), cobra.ShellCompDirectiveNoFileComp
	}))
}

// AllowedOutputFormats returns the allowed formats for the output flag.
func (o *Options) AllowedOutputFormats() []string {
	return []string{"json", "yaml"}
}

// PrintObject prints an object to IOStreams.out, using o.Output to print in the selected output format.
func (o *Options) PrintObject(obj interface{}) error {
	switch o.Output {
	case "":
		if _, ok := obj.(fmt.Stringer); ok {
			_, err := fmt.Fprintf(o.IOStreams.Out, "%s", obj)
			return err
		}

		_, err := fmt.Fprintf(o.IOStreams.Out, "%v", obj)

		return err
	case "yaml":
		marshalled, err := yaml.Marshal(&obj)
		if err != nil {
			return err
		}

		_, err = fmt.Fprint(o.IOStreams.Out, string(marshalled))

		return err
	case "json":
		marshalled, err := json.MarshalIndent(&obj, "", "  ")
		if err != nil {
			return err
		}

		_, err = fmt.Fprintln(o.IOStreams.Out, string(marshalled))

		return err
	default:
		// There is a bug in the program if we hit this case.
		// However, we follow a policy of never panicking.
		return fmt.Errorf("options were not validated: --output=%q should have been rejected", o.Output)
	}
}

// Validate validates the provided options.
func (o *Options) Validate() error {
	if o.Output != "" && o.Output != "yaml" && o.Output != "json" {
		return errors.New("--output must be either 'yaml' or 'json'")
	}

	return nil
}

// Complete adapts from the command line args to the data required.
func (o *Options) Complete(_ util.Factory, _ *cobra.Command, _ []string) error {
	return nil
}

// Run does the actual work of the command.
func (o *Options) Run(util.Factory) error {
	return errors.New("method \"Run\" not implemented")
}
