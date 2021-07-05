/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package root

import (
	"context"
	"fmt"
	"os"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/target"

	"github.com/spf13/cobra"
)

type cobraCompletionFunc func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)
type cobraCompletionFuncWithError func(ctx context.Context, manager target.Manager) ([]string, error)

func completionWrapper(f *util.FactoryImpl, completer cobraCompletionFuncWithError) cobraCompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// By default, the factory will provide a target manager that uses the DynamicTargetProvider (DTP)
		// implementation, i.e. is based on the file just as much as the CLI flags.
		// The DTP tries to allow users to "move up", i.e. when they already targeted a shoot, just adding
		// "--garden foo" should not just change the used garden cluster, but _target_ the garden (instead
		// of the shoot). This behaviour is not suitable for the CLI completion functions, because
		// when completing "gardenctl --garden foo --shoot [tab]", the DTP would consider this as
		// "user wants to target the garden" and will therefore throw away the project/seed information.
		// Project and seed information however are important for the completion functions.
		//
		// To work around this, all completion functions use the regular filesystem based target provider.
		f.TargetFile = targetProvider.TargetFile
		f.TargetProvider = nil

		manager, err := f.Manager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		result, err := completer(f.Context(), manager)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		return util.FilterStringsByPrefix(toComplete, result), cobra.ShellCompDirectiveNoFileComp
	}
}

func gardenFlagCompletionFunc(ctx context.Context, manager target.Manager) ([]string, error) {
	return util.GardenNames(manager)
}

func projectFlagCompletionFunc(ctx context.Context, manager target.Manager) ([]string, error) {
	// any --garden flag has precedence over the config file
	var currentTarget target.Target

	if targetProvider.GardenNameFlag != "" {
		currentTarget = target.NewTarget(targetProvider.GardenNameFlag, "", "", "")
	} else {
		var err error

		currentTarget, err = manager.CurrentTarget()
		if err != nil {
			return nil, fmt.Errorf("failed to read current target: %w", err)
		}
	}

	return util.ProjectNamesForTarget(ctx, manager, currentTarget)
}

func seedFlagCompletionFunc(ctx context.Context, manager target.Manager) ([]string, error) {
	// any --garden flag has precedence over the config file
	var currentTarget target.Target

	if targetProvider.GardenNameFlag != "" {
		currentTarget = target.NewTarget(targetProvider.GardenNameFlag, "", "", "")
	} else {
		var err error
		currentTarget, err = manager.CurrentTarget()
		if err != nil {
			return nil, fmt.Errorf("failed to read current target: %w", err)
		}
	}

	return util.SeedNamesForTarget(ctx, manager, currentTarget)
}

func shootFlagCompletionFunc(ctx context.Context, manager target.Manager) ([]string, error) {
	// errors are okay here, as we patch the target anyway
	currentTarget, _ := manager.CurrentTarget()

	if targetProvider.GardenNameFlag != "" {
		currentTarget = currentTarget.WithGardenName(targetProvider.GardenNameFlag)
	}

	if targetProvider.ProjectNameFlag != "" {
		currentTarget = currentTarget.WithProjectName(targetProvider.ProjectNameFlag).WithSeedName("")
	} else if targetProvider.SeedNameFlag != "" {
		currentTarget = currentTarget.WithSeedName(targetProvider.SeedNameFlag).WithProjectName("")
	}

	return util.ShootNamesForTarget(ctx, manager, currentTarget)
}
