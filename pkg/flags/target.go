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

func RegisterTargetFlagCompletionFuncs(cmd *cobra.Command, factory util.Factory, ioStreams util.IOStreams, flags *pflag.FlagSet) {
	utilruntime.Must(cmd.RegisterFlagCompletionFunc("garden", completionWrapper(factory, ioStreams, gardenFlagCompletionFunc)))
	utilruntime.Must(cmd.RegisterFlagCompletionFunc("project", completionWrapper(factory, ioStreams, projectFlagCompletionFunc)))
	utilruntime.Must(cmd.RegisterFlagCompletionFunc("seed", completionWrapper(factory, ioStreams, seedFlagCompletionFunc)))
	utilruntime.Must(cmd.RegisterFlagCompletionFunc("shoot", completionWrapper(factory, ioStreams, shootFlagCompletionFunc)))
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
