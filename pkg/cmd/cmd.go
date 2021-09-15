/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gardener/gardenctl-v2/internal/util"
	cmdssh "github.com/gardener/gardenctl-v2/pkg/cmd/ssh"
	cmdtarget "github.com/gardener/gardenctl-v2/pkg/cmd/target"
	cmdversion "github.com/gardener/gardenctl-v2/pkg/cmd/version"
	"github.com/gardener/gardenctl-v2/pkg/target"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
)

const (
	envPrefix        = "GCTL"
	envGardenHomeDir = envPrefix + "_HOME"
	envConfigName    = envPrefix + "_CONFIG_NAME"

	gardenHomeFolder = ".garden"
	configName       = "gardenctl-v2"
	targetFilename   = "target.yaml"
)

var (
	targetProvider = &target.DynamicTargetProvider{}
	factory        = util.FactoryImpl{
		TargetProvider: targetProvider,
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	ioStreams := util.NewIOStreams()

	rootCmd := &cobra.Command{
		Use:          "gardenctl",
		Short:        "gardenctl is a utility to interact with Gardener installations",
		SilenceUsage: true,
	}

	rootCmd.AddCommand(cmdssh.NewCmdSSH(&factory, cmdssh.NewSSHOptions(ioStreams)))
	rootCmd.AddCommand(cmdtarget.NewCmdTarget(&factory, cmdtarget.NewTargetOptions(ioStreams), targetProvider))
	rootCmd.AddCommand(cmdversion.NewCmdVersion(&factory, cmdversion.NewVersionOptions(ioStreams)))

	// Do not precalculate what $HOME is for the help text, because it prevents
	// usage where the current user has no home directory (which might _just_ be
	// the reason the user chose to specify an explicit config file).
	rootCmd.PersistentFlags().StringVar(&factory.ConfigFile, "config", "", fmt.Sprintf("config file (default is $HOME/%s/%s.yaml)", gardenHomeFolder, configName))

	// allow to temporarily re-target a different cluster
	rootCmd.PersistentFlags().StringVar(&targetProvider.GardenNameFlag, "garden", "", "target the given garden cluster")
	rootCmd.PersistentFlags().StringVar(&targetProvider.ProjectNameFlag, "project", "", "target the given project")
	rootCmd.PersistentFlags().StringVar(&targetProvider.SeedNameFlag, "seed", "", "target the given seed cluster")
	rootCmd.PersistentFlags().StringVar(&targetProvider.ShootNameFlag, "shoot", "", "target the given shoot cluster")

	utilruntime.Must(rootCmd.RegisterFlagCompletionFunc("garden", completionWrapper(&factory, gardenFlagCompletionFunc)))
	utilruntime.Must(rootCmd.RegisterFlagCompletionFunc("project", completionWrapper(&factory, projectFlagCompletionFunc)))
	utilruntime.Must(rootCmd.RegisterFlagCompletionFunc("seed", completionWrapper(&factory, seedFlagCompletionFunc)))
	utilruntime.Must(rootCmd.RegisterFlagCompletionFunc("shoot", completionWrapper(&factory, shootFlagCompletionFunc)))

	cobra.OnInitialize(initConfig)

	// any error would already be printed, so avoid doing it again here
	if rootCmd.Execute() != nil {
		os.Exit(1)
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	var err error

	if factory.ConfigFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(factory.ConfigFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		cobra.CheckErr(err)

		configPath := filepath.Join(home, gardenHomeFolder)

		// Search config in $HOME/.garden or in path provided with the env variable GCTL_HOME with name ".garden-login" (without extension) or name from env variable GCTL_CONFIG_NAME.
		envHomeDir, err := homedir.Expand(os.Getenv(envGardenHomeDir))
		cobra.CheckErr(err)

		viper.AddConfigPath(envHomeDir)
		viper.AddConfigPath(configPath)
		if os.Getenv(envConfigName) != "" {
			viper.SetConfigName(os.Getenv(envConfigName))
		} else {
			viper.SetConfigName(configName)
		}
	}

	viper.SetEnvPrefix(envPrefix)
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		klog.Errorf("failed to read config file: %v", err)
	}

	// initialize the factory

	// prefer an explicit GCTL_HOME env,
	// but fallback to the system-defined home directory
	home := os.Getenv(envGardenHomeDir)
	if len(home) == 0 {
		home, err = homedir.Dir()
		cobra.CheckErr(err)

		home = filepath.Join(home, gardenHomeFolder)
	}

	factory.ConfigFile = viper.ConfigFileUsed()
	targetProvider.TargetFile = filepath.Join(home, targetFilename)
	factory.GardenHomeDirectory = home
}

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
