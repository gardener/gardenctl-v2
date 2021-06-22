/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package root

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/ssh"
	"github.com/gardener/gardenctl-v2/pkg/cmd/target"
	"github.com/gardener/gardenctl-v2/pkg/cmd/version"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/cli-runtime/pkg/genericclioptions"
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
	targetProvider = &DynamicTargetProvider{}
	factory        = util.FactoryImpl{
		TargetProvider: targetProvider,
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	ioStreams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	rootCmd := &cobra.Command{
		Use:          "gardenctl",
		Short:        "gardenctl is a utility to interact with Gardener installations",
		SilenceUsage: true,
	}

	rootCmd.AddCommand(ssh.NewCommand(&factory, ssh.NewOptions(ioStreams)))
	rootCmd.AddCommand(target.NewCommand(&factory, target.NewOptions(ioStreams)))
	rootCmd.AddCommand(version.NewCommand(&factory, version.NewOptions(ioStreams)))

	// Do not precalculate what $HOME is for the help text, because it prevents
	// usage where the current user has no home directory (which might _just_ be
	// the reason the user chose to specify an explicit config file).
	rootCmd.PersistentFlags().StringVar(&factory.ConfigFile, "config", "", fmt.Sprintf("config file (default is $HOME/%s/%s.yaml)", gardenHomeFolder, configName))

	// allow to temporarily re-target a different cluster
	rootCmd.PersistentFlags().StringVar(&targetProvider.GardenNameFlag, "garden", "", "target the given garden cluster")
	rootCmd.PersistentFlags().StringVar(&targetProvider.ProjectNameFlag, "project", "", "target the given project")
	rootCmd.PersistentFlags().StringVar(&targetProvider.SeedNameFlag, "seed", "", "target the given seed cluster")
	rootCmd.PersistentFlags().StringVar(&targetProvider.ShootNameFlag, "shoot", "", "target the given shoot cluster")

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
