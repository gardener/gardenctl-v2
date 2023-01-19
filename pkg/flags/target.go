/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

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

// RegisterCompletionFuncsForTargetFlags registers the completion functions to a given cobra command
// for the target flags (--garden, --project, --seed and --shoot). Each completion function is only
// registered if the flag has been previously added to the provided flag set.
func RegisterCompletionFuncsForTargetFlags(cmd *cobra.Command, factory util.Factory, ioStreams util.IOStreams, flags *pflag.FlagSet) {
	if cmd.Flag("garden") != nil {
		utilruntime.Must(cmd.RegisterFlagCompletionFunc("garden", completionWrapper(factory, ioStreams, gardenFlagCompletionFunc)))
	}

	if cmd.Flag("project") != nil {
		utilruntime.Must(cmd.RegisterFlagCompletionFunc("project", completionWrapper(factory, ioStreams, projectFlagCompletionFunc)))
	}

	if cmd.Flag("seed") != nil {
		utilruntime.Must(cmd.RegisterFlagCompletionFunc("seed", completionWrapper(factory, ioStreams, seedFlagCompletionFunc)))
	}

	if cmd.Flag("shoot") != nil {
		utilruntime.Must(cmd.RegisterFlagCompletionFunc("shoot", completionWrapper(factory, ioStreams, shootFlagCompletionFunc)))
	}
}

type (
	cobraCompletionFunc          func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)
	cobraCompletionFuncWithError func(ctx context.Context, manager target.Manager) ([]string, error)
)

func completionWrapper(factory util.Factory, ioStreams util.IOStreams, completionFunc cobraCompletionFuncWithError) cobraCompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		manager, err := factory.Manager()
		if err != nil {
			fmt.Fprintf(ioStreams.ErrOut, "%v\n", err)
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		result, err := completionFunc(factory.Context(), manager)
		if err != nil {
			fmt.Fprintf(ioStreams.ErrOut, "%v\n", err)
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		return util.FilterStringsByPrefix(toComplete, result), cobra.ShellCompDirectiveNoFileComp
	}
}

func gardenFlagCompletionFunc(_ context.Context, manager target.Manager) ([]string, error) {
	return manager.GardenNames()
}

func projectFlagCompletionFunc(ctx context.Context, manager target.Manager) ([]string, error) {
	return manager.ProjectNames(ctx)
}

func seedFlagCompletionFunc(ctx context.Context, manager target.Manager) ([]string, error) {
	return manager.SeedNames(ctx)
}

func shootFlagCompletionFunc(ctx context.Context, manager target.Manager) ([]string, error) {
	return manager.ShootNames(ctx)
}
