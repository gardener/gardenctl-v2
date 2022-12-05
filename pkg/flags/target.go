/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package flags

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

func AddTargetFlags(cmd *cobra.Command, f util.Factory, ioStreams util.IOStreams, flags *pflag.FlagSet) {
	manager, err := f.Manager()
	utilruntime.Must(err)

	// TODO: the flags are defined in the TargetFlags(Iml) struct. Maybe it makes sense to define those flags here
	// along with the completions for them. But currently the struct fields are private and hence we could not bind
	// the fields to the flags. Eventually we could make those exported.
	tf := manager.TargetFlags()
	tf.AddFlags(flags)

	utilruntime.Must(cmd.RegisterFlagCompletionFunc("garden", completionWrapper(f, ioStreams, gardenFlagCompletionFunc)))
	utilruntime.Must(cmd.RegisterFlagCompletionFunc("project", completionWrapper(f, ioStreams, projectFlagCompletionFunc)))
	utilruntime.Must(cmd.RegisterFlagCompletionFunc("seed", completionWrapper(f, ioStreams, seedFlagCompletionFunc)))
	utilruntime.Must(cmd.RegisterFlagCompletionFunc("shoot", completionWrapper(f, ioStreams, shootFlagCompletionFunc)))
}

type cobraCompletionFunc func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)
type cobraCompletionFuncWithError func(ctx context.Context, manager target.Manager) ([]string, error)

func completionWrapper(factory util.Factory, ioStreams util.IOStreams, completerFunc cobraCompletionFuncWithError) cobraCompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		manager, err := factory.Manager()

		if err != nil {
			fmt.Fprintf(ioStreams.ErrOut, "%v\n", err)
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		result, err := completerFunc(factory.Context(), manager)
		if err != nil {
			fmt.Fprintf(ioStreams.ErrOut, "%v\n", err)
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		return util.FilterStringsByPrefix(toComplete, result), cobra.ShellCompDirectiveNoFileComp
	}
}

// TODO: util.Gardennames etc. are currently used here and in the package cmd/target. Maybe we can move it out of the util package?
func gardenFlagCompletionFunc(_ context.Context, manager target.Manager) ([]string, error) {
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
