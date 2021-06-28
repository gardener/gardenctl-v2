/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package root

import (
	"fmt"
	"os"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/target"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/sets"
)

func newCompletionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate completion script",
		Long: `To load completions:

	Bash:

	  $ source <(gardenctl completion bash)

	  # To load completions for each session, execute once:
	  # Linux:
	  $ gardenctl completion bash > /etc/bash_completion.d/gardenctl
	  # macOS:
	  $ gardenctl completion bash > /usr/local/etc/bash_completion.d/gardenctl

	Zsh:

	  # If shell completion is not already enabled in your environment,
	  # you will need to enable it.  You can execute the following once:

	  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

	  # To load completions for each session, execute once:
	  $ gardenctl completion zsh > "${fpath[1]}/_gardenctl"

	  # You will need to start a new shell for this setup to take effect.

	fish:

	  $ gardenctl completion fish | source

	  # To load completions for each session, execute once:
	  $ gardenctl completion fish > ~/.config/fish/completions/gardenctl.fish

	PowerShell:

	  PS> gardenctl completion powershell | Out-String | Invoke-Expression

	  # To load completions for every new session, run:
	  PS> gardenctl completion powershell > gardenctl.ps1
	  # and source this file from your PowerShell profile.
	`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.ExactValidArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			}

			return nil
		},
	}
}

type cobraCompletionFunc func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)
type cobraCompletionFuncWithError func(f util.Factory) ([]string, error)

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

		result, err := completer(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		return util.FilterStringsByPrefix(toComplete, result), cobra.ShellCompDirectiveNoFileComp
	}
}

func gardenFlagCompletionFunc(f util.Factory) ([]string, error) {
	manager, err := f.Manager()
	if err != nil {
		return nil, fmt.Errorf("failed to create manager: %w", err)
	}

	names := sets.NewString()
	for _, garden := range manager.Configuration().Gardens {
		names.Insert(garden.Name)
	}

	return names.List(), nil
}

func projectFlagCompletionFunc(f util.Factory) ([]string, error) {
	manager, err := f.Manager()
	if err != nil {
		return nil, fmt.Errorf("failed to create manager: %w", err)
	}

	// any --garden flag has precedence over the config file
	var currentTarget target.Target

	if targetProvider.GardenNameFlag != "" {
		currentTarget = target.NewTarget(targetProvider.GardenNameFlag, "", "", "")
	} else {
		currentTarget, err = manager.CurrentTarget()
		if err != nil {
			return nil, fmt.Errorf("failed to read current target: %w", err)
		}
	}

	gardenClient, err := manager.GardenClient(currentTarget)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client for garden cluster %q: %w", currentTarget.GardenName(), err)
	}

	projectList := &gardencorev1beta1.ProjectList{}
	if err := gardenClient.List(f.Context(), projectList); err != nil {
		return nil, fmt.Errorf("failed to list projects on garden cluster %q: %w", currentTarget.GardenName(), err)
	}

	names := sets.NewString()
	for _, project := range projectList.Items {
		names.Insert(project.Name)
	}

	return names.List(), nil
}

func seedFlagCompletionFunc(f util.Factory) ([]string, error) {
	manager, err := f.Manager()
	if err != nil {
		return nil, fmt.Errorf("failed to create manager: %w", err)
	}

	// any --garden flag has precedence over the config file
	var currentTarget target.Target

	if targetProvider.GardenNameFlag != "" {
		currentTarget = target.NewTarget(targetProvider.GardenNameFlag, "", "", "")
	} else {
		currentTarget, err = manager.CurrentTarget()
		if err != nil {
			return nil, fmt.Errorf("failed to read current target: %w", err)
		}
	}

	gardenClient, err := manager.GardenClient(currentTarget)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client for garden cluster %q: %w", currentTarget.GardenName(), err)
	}

	seedList := &gardencorev1beta1.SeedList{}
	if err := gardenClient.List(f.Context(), seedList); err != nil {
		return nil, fmt.Errorf("failed to list seeds on garden cluster %q: %w", currentTarget.GardenName(), err)
	}

	names := sets.NewString()
	for _, seed := range seedList.Items {
		names.Insert(seed.Name)
	}

	return names.List(), nil
}

func shootFlagCompletionFunc(f util.Factory) ([]string, error) {
	manager, err := f.Manager()
	if err != nil {
		return nil, fmt.Errorf("failed to create manager: %w", err)
	}

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

	gardenClient, err := manager.GardenClient(currentTarget)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client for garden cluster %q: %w", currentTarget.GardenName(), err)
	}

	ctx := f.Context()

	shoots, err := util.ShootsForTarget(ctx, gardenClient, currentTarget)
	if err != nil {
		return nil, fmt.Errorf("failed to list shoots on garden cluster %q: %w", currentTarget.GardenName(), err)
	}

	names := sets.NewString()
	for _, shoot := range shoots {
		names.Insert(shoot.Name)
	}

	return names.List(), nil
}
