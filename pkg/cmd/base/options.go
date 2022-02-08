/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package base

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardenctl-v2/pkg/target"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"

	"github.com/gardener/gardenctl-v2/internal/util"
)

//go:generate mockgen -destination=./mocks/mock_options.go -package=mocks github.com/gardener/gardenctl-v2/pkg/cmd/base CommandOptions

// CommandOptions is the base interface for command options.
type CommandOptions interface {
	// Complete adapts from the command line args to the data required.
	Complete(util.Factory, *cobra.Command, []string) error
	// Validate validates the provided options.
	Validate() error
	// Run does the actual work of the command.
	Run(util.Factory) error
	// AddOutputFlags adds flags to adjust the output to a cobra command.
	AddOutputFlags(*pflag.FlagSet)
	// AddTargetOverrideFlags adds flags to adjust the output to a cobra command.
	AddTargetOverrideFlags(f util.Factory, flags *pflag.FlagSet)
	// RegisterTargetFlagCompletions registers completions for all target flags added to this command.
	RegisterTargetFlagCompletions(f util.Factory, cmd *cobra.Command, ioStreams util.IOStreams)
}

// Options contains all settings that are used across all commands in gardenctl.
type Options struct {
	// IOStreams provides the standard names for iostreams
	IOStreams util.IOStreams

	// Output defines the output format of the version information. Either 'yaml' or 'json'
	Output string
}

var _ CommandOptions = &Options{}

// WrapRunE creates a cobra RunE function that has access to the factory
func WrapRunE(o CommandOptions, f util.Factory) func(cmd *cobra.Command, args []string) error {
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

type cobraCompletionFunc func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)
type cobraCompletionFuncWithError func(ctx context.Context, manager target.Manager) ([]string, error)

// WrapCompletionFunction creates a function that can be used to register a cobra completion command that has access to the manager
func WrapCompletionFunction(f util.Factory, ioStreams util.IOStreams, completer cobraCompletionFuncWithError) cobraCompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		manager, err := f.Manager()

		if err != nil {
			fmt.Fprintf(ioStreams.ErrOut, "%v\n", err)
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		result, err := completer(f.Context(), manager)
		if err != nil {
			fmt.Fprintf(ioStreams.ErrOut, "%v\n", err)
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		return util.FilterStringsByPrefix(toComplete, result), cobra.ShellCompDirectiveNoFileComp
	}
}

// NewOptions returns initialized Options
func NewOptions(ioStreams util.IOStreams) *Options {
	return &Options{
		IOStreams: ioStreams,
	}
}

// AddOutputFlags adds flags to adjust the output to a cobra command
func (o *Options) AddOutputFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&o.Output, "output", "o", o.Output, "One of 'yaml' or 'json'.")
}

// AddTargetOverrideFlags adds flags to override the current target to a cobra command
func (o *Options) AddTargetOverrideFlags(f util.Factory, flags *pflag.FlagSet) {
	f.TF().AddOverrideFlags(flags)
}

// RegisterTargetFlagCompletions registers completions for all target flags added to this command.
func (o *Options) RegisterTargetFlagCompletions(f util.Factory, cmd *cobra.Command, ioStreams util.IOStreams) {
	if cmd.Flag("garden") != nil {
		utilruntime.Must(cmd.RegisterFlagCompletionFunc("garden", WrapCompletionFunction(f, ioStreams, gardenFlagCompletionFunc)))
	}
	if cmd.Flag("project") != nil {
		utilruntime.Must(cmd.RegisterFlagCompletionFunc("project", WrapCompletionFunction(f, ioStreams, projectFlagCompletionFunc)))
	}
	if cmd.Flag("seed") != nil {
		utilruntime.Must(cmd.RegisterFlagCompletionFunc("seed", WrapCompletionFunction(f, ioStreams, seedFlagCompletionFunc)))
	}
	if cmd.Flag("shoot") != nil {
		utilruntime.Must(cmd.RegisterFlagCompletionFunc("shoot", WrapCompletionFunction(f, ioStreams, shootFlagCompletionFunc)))
	}
}

func gardenFlagCompletionFunc(ctx context.Context, manager target.Manager) ([]string, error) {
	return util.GardenNames(manager)
}

func projectFlagCompletionFunc(ctx context.Context, manager target.Manager) ([]string, error) {
	return util.ProjectNamesForTarget(ctx, manager)
}

func seedFlagCompletionFunc(ctx context.Context, manager target.Manager) ([]string, error) {
	return util.SeedNamesForTarget(ctx, manager)
}

func shootFlagCompletionFunc(ctx context.Context, manager target.Manager) ([]string, error) {
	return util.ShootNamesForTarget(ctx, manager)
}

// PrintObject prints an object to IOStreams.out, using o.Output to print in the selected output format
func (o *Options) PrintObject(obj interface{}) error {
	switch o.Output {
	case "":
		fmt.Fprintf(o.IOStreams.Out, "%v", obj)

	case "yaml":
		marshalled, err := yaml.Marshal(&obj)
		if err != nil {
			return err
		}

		fmt.Fprintln(o.IOStreams.Out, string(marshalled))

	case "json":
		marshalled, err := json.MarshalIndent(&obj, "", "  ")
		if err != nil {
			return err
		}

		fmt.Fprintln(o.IOStreams.Out, string(marshalled))

	default:
		// There is a bug in the program if we hit this case.
		// However, we follow a policy of never panicking.
		return fmt.Errorf("options were not validated: --output=%q should have been rejected", o.Output)
	}

	return nil
}

// Validate validates the provided options.
func (o *Options) Validate() error {
	if o.Output != "" && o.Output != "yaml" && o.Output != "json" {
		return errors.New("--output must be either 'yaml' or 'json'")
	}

	return nil
}

// Complete adapts from the command line args to the data required.
func (o *Options) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	return nil
}

// Run does the actual work of the command.
func (o *Options) Run(util.Factory) error {
	return errors.New("method \"Run\" not implemented")
}
