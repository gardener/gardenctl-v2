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
	targetFlags = target.NewTargetFlags("", "", "", "")
	factory     = util.FactoryImpl{}
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	ioStreams := util.NewIOStreams()

	cmd := &cobra.Command{
		Use:          "gardenctl",
		Short:        "gardenctl is a utility to interact with Gardener installations",
		SilenceUsage: true,
	}

	cmd.AddCommand(cmdssh.NewCmdSSH(&factory, cmdssh.NewSSHOptions(ioStreams)))
	cmd.AddCommand(cmdtarget.NewCmdTarget(&factory, cmdtarget.NewTargetOptions(ioStreams)))
	cmd.AddCommand(cmdversion.NewCmdVersion(&factory, cmdversion.NewVersionOptions(ioStreams)))

	flags := cmd.PersistentFlags()
	// Do not precalculate what $HOME is for the help text, because it prevents
	// usage where the current user has no home directory (which might _just_ be
	// the reason the user chose to specify an explicit config file).
	flags.StringVar(&factory.ConfigFile, "config", "", fmt.Sprintf("config file (default is $HOME/%s/%s.yaml)", gardenHomeFolder, configName))

	// allow to temporarily re-target a different cluster
	targetFlags.AddFlags(cmd)

	cobra.OnInitialize(initConfig, func() {
		registerFlagCompletions(cmd)
	})

	// any error would already be printed, so avoid doing it again here
	if cmd.Execute() != nil {
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
	factory.GardenHomeDirectory = home
	targetFile := filepath.Join(home, targetFilename)
	factory.TargetFile = targetFile
	factory.TargetProvider = target.NewDynamicTargetProvider(targetFile, targetFlags)
}

// registerFlagCompletions register completion functions for cobra flags
func registerFlagCompletions(cmd *cobra.Command) {
	f := util.FactoryImpl{
		GardenHomeDirectory: factory.GardenHomeDirectory,
		ConfigFile:          factory.ConfigFile,
		TargetFile:          factory.TargetFile,
		TargetProvider:      nil,
	}
	if manager, err := f.Manager(); err != nil {
		klog.Errorf("failed to create target manager: %v", err)
	} else {
		utilruntime.Must(cmd.RegisterFlagCompletionFunc("garden", completionWrapper(f.Context(), manager, gardenFlagCompletionFunc)))
		utilruntime.Must(cmd.RegisterFlagCompletionFunc("project", completionWrapper(f.Context(), manager, projectFlagCompletionFunc)))
		utilruntime.Must(cmd.RegisterFlagCompletionFunc("seed", completionWrapper(f.Context(), manager, seedFlagCompletionFunc)))
		utilruntime.Must(cmd.RegisterFlagCompletionFunc("shoot", completionWrapper(f.Context(), manager, shootFlagCompletionFunc)))
	}
}

type cobraCompletionFunc func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)
type cobraCompletionFuncWithError func(ctx context.Context, manager target.Manager) ([]string, error)

func completionWrapper(ctx context.Context, manager target.Manager, completer cobraCompletionFuncWithError) cobraCompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		result, err := completer(ctx, manager)
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

	tf := manager.TargetFlags()

	if tf.GardenName() != "" {
		currentTarget = target.NewTarget(tf.GardenName(), "", "", "")
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

	tf := manager.TargetFlags()

	if tf.GardenName() != "" {
		currentTarget = target.NewTarget(tf.GardenName(), "", "", "")
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

	tf := manager.TargetFlags()

	if tf.GardenName() != "" {
		currentTarget = currentTarget.WithGardenName(tf.GardenName())
	}

	if tf.ProjectName() != "" {
		currentTarget = currentTarget.WithProjectName(tf.ProjectName()).WithSeedName("")
	} else if tf.SeedName() != "" {
		currentTarget = currentTarget.WithSeedName(tf.SeedName()).WithProjectName("")
	}

	return util.ShootNamesForTarget(ctx, manager, currentTarget)
}
