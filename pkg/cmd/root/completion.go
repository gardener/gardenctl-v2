/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package root

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
type cobraCompletionFuncWithError func(cmd *cobra.Command, args []string, toComplete string) ([]string, error)

func completionWrapper(completer cobraCompletionFuncWithError) cobraCompletionFunc {
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
		factory.TargetFile = targetProvider.TargetFile
		factory.TargetProvider = nil

		result, err := completer(cmd, args, toComplete)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return nil, cobra.ShellCompDirectiveDefault
		}

		if toComplete != "" {
			filtered := []string{}

			for _, item := range result {
				if strings.HasPrefix(item, toComplete) {
					filtered = append(filtered, item)
				}
			}

			result = filtered
		}

		return result, cobra.ShellCompDirectiveDefault
	}
}

func gardenFlagCompletionFunc(cmd *cobra.Command, args []string, toComplete string) ([]string, error) {
	cfg, err := config.LoadFromFile(factory.ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	names := sets.NewString()
	for _, garden := range cfg.Gardens {
		names.Insert(garden.Name)
	}

	return names.List(), nil
}

func projectFlagCompletionFunc(cmd *cobra.Command, args []string, toComplete string) ([]string, error) {
	manager, err := factory.Manager()
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
	if err := gardenClient.List(context.Background(), projectList); err != nil {
		return nil, fmt.Errorf("failed to list projects on garden cluster %q: %w", currentTarget.GardenName(), err)
	}

	names := sets.NewString()
	for _, project := range projectList.Items {
		names.Insert(project.Name)
	}

	return names.List(), nil
}

func seedFlagCompletionFunc(cmd *cobra.Command, args []string, toComplete string) ([]string, error) {
	manager, err := factory.Manager()
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
	if err := gardenClient.List(context.Background(), seedList); err != nil {
		return nil, fmt.Errorf("failed to list seeds on garden cluster %q: %w", currentTarget.GardenName(), err)
	}

	names := sets.NewString()
	for _, seed := range seedList.Items {
		names.Insert(seed.Name)
	}

	return names.List(), nil
}

func shootFlagCompletionFunc(cmd *cobra.Command, args []string, toComplete string) ([]string, error) {
	manager, err := factory.Manager()
	if err != nil {
		return nil, fmt.Errorf("failed to create manager: %w", err)
	}

	// for simplicity, always try to read the current target file and ignore any errors
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

	ctx := context.Background()

	var listOpt client.ListOption

	if currentTarget.ProjectName() != "" {
		project, err := util.ProjectForTarget(ctx, gardenClient, currentTarget)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch project: %w", err)
		}

		if project.Spec.Namespace == nil {
			return nil, nil
		}

		listOpt = &client.ListOptions{Namespace: *project.Spec.Namespace}
	} else if currentTarget.SeedName() != "" {
		listOpt = client.MatchingFields{gardencore.ShootSeedName: currentTarget.SeedName()}
	} else {
		return nil, nil
	}

	shootList := &gardencorev1beta1.ShootList{}
	if err := gardenClient.List(ctx, shootList, listOpt); err != nil {
		return nil, fmt.Errorf("failed to list shoots on garden cluster %q: %w", currentTarget.GardenName(), err)
	}

	names := sets.NewString()
	for _, shoot := range shootList.Items {
		names.Insert(shoot.Name)
	}

	return names.List(), nil
}
