/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"

	"github.com/gardener/gardenctl-v2/internal/util"
	cmdconfig "github.com/gardener/gardenctl-v2/pkg/cmd/config"
	cmdenv "github.com/gardener/gardenctl-v2/pkg/cmd/env"
	"github.com/gardener/gardenctl-v2/pkg/cmd/kubeconfig"
	cmdssh "github.com/gardener/gardenctl-v2/pkg/cmd/ssh"
	cmdtarget "github.com/gardener/gardenctl-v2/pkg/cmd/target"
	cmdversion "github.com/gardener/gardenctl-v2/pkg/cmd/version"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

const (
	envPrefix        = "GCTL"
	envGardenHomeDir = envPrefix + "_HOME"
	envConfigName    = envPrefix + "_CONFIG_NAME"

	gardenHomeFolder = ".garden"
	configName       = "gardenctl-v2"
	configExtension  = "yaml"
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the root cmd.
func Execute() {
	cmd := NewDefaultGardenctlCommand()
	// any error would already be printed, so avoid doing it again here
	if cmd.Execute() != nil {
		os.Exit(1)
	}
}

// NewDefaultGardenctlCommand creates the `gardenctl` command with defaults
func NewDefaultGardenctlCommand() *cobra.Command {
	factory := util.FactoryImpl{
		TargetFlags: target.NewTargetFlags("", "", "", "", false),
	}
	ioStreams := util.NewIOStreams()

	return NewGardenctlCommand(&factory, ioStreams)
}

// NewGardenctlCommand creates the `gardenctl` command
func NewGardenctlCommand(f *util.FactoryImpl, ioStreams util.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gardenctl",
		Short: "Gardenctl is a utility to interact with Gardener installations",
		Long: `Gardenctl is a utility to interact with Gardener installations.

The state of gardenctl is bound to a shell session and is not shared across windows, tabs or panes.
A shell session is defined by the environment variable GCTL_SESSION_ID. If this is not defined,
the value of the TERM_SESSION_ID environment variable is used instead. If both are not defined,
this leads to an error and gardenctl cannot be executed. The target.yaml and temporary
kubeconfig.*.yaml files are store in the following directory ${TMPDIR}/garden/${GCTL_SESSION_ID}.

You can make sure that GCTL_SESSION_ID or TERM_SESSION_ID is always present by adding
the following code to your terminal profile ~/.profile, ~/.bashrc or comparable file.
  bash and zsh:

      [ -n "$GCTL_SESSION_ID" ] || [ -n "$TERM_SESSION_ID" ] || export GCTL_SESSION_ID=$(uuidgen)

  fish:

      [ -n "$GCTL_SESSION_ID" ] || [ -n "$TERM_SESSION_ID" ] || set -gx GCTL_SESSION_ID (uuidgen)

  powershell:

      if ( !(Test-Path Env:GCTL_SESSION_ID) -and !(Test-Path Env:TERM_SESSION_ID) ) { $Env:GCTL_SESSION_ID = [guid]::NewGuid().ToString() }

Find more information at: https://github.com/gardener/gardenctl-v2/blob/master/README.md
`,
		SilenceUsage: true,
	}

	cmd.SetIn(ioStreams.In)
	cmd.SetOut(ioStreams.Out)
	cmd.SetErr(ioStreams.ErrOut)

	// register initializers
	cobra.OnInitialize(func() {
		initConfig(f)
	})

	flags := cmd.PersistentFlags()

	// Normalize all flags that are coming from other packages or pre-configurations
	// a.k.a. change all "_" to "-". e.g. klog package
	flags.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)

	addKlogFlags(flags)

	// Do not precalculate what $HOME is for the help text, because it prevents
	// usage where the current user has no home directory (which might _just_ be
	// the reason the user chose to specify an explicit config file).
	flags.StringVar(&f.ConfigFile, "config", "", fmt.Sprintf("config file (default is %s)", filepath.Join("~", gardenHomeFolder, configName+"."+configExtension)))

	// allow to temporarily re-target a different cluster
	f.TargetFlags.AddFlags(flags)

	registerCompletionFuncForGlobalFlags(cmd, f, ioStreams)

	// add subcommands
	cmd.AddCommand(cmdssh.NewCmdSSH(f, cmdssh.NewSSHOptions(ioStreams)))
	cmd.AddCommand(cmdtarget.NewCmdTarget(f, ioStreams))
	cmd.AddCommand(cmdversion.NewCmdVersion(f, cmdversion.NewVersionOptions(ioStreams)))
	cmd.AddCommand(cmdconfig.NewCmdConfig(f, ioStreams))
	cmd.AddCommand(cmdenv.NewCmdProviderEnv(f, ioStreams))
	cmd.AddCommand(cmdenv.NewCmdKubectlEnv(f, ioStreams))
	cmd.AddCommand(cmdenv.NewCmdRC(f, ioStreams))
	cmd.AddCommand(kubeconfig.NewCmdKubeconfig(f, ioStreams))

	return cmd
}

// initConfig reads in config file and ENV variables if set.
func initConfig(f *util.FactoryImpl) {
	var configFile string

	if f.ConfigFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(f.ConfigFile)
		configFile = f.ConfigFile
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		cobra.CheckErr(err)

		configPath := filepath.Join(home, gardenHomeFolder)
		configFile = configPath

		// Search config in ~/.garden or in path provided with the env variable GCTL_HOME with name "gardenctl-v2" (without extension) or name from env variable GCTL_CONFIG_NAME.
		envHomeDir, ok := os.LookupEnv(envGardenHomeDir)
		if ok {
			envHomeDir, err = homedir.Expand(envHomeDir)
			cobra.CheckErr(err)
			configFile = envHomeDir
			viper.AddConfigPath(envHomeDir)
		}

		viper.AddConfigPath(configPath)

		if name, ok := os.LookupEnv(envConfigName); ok {
			viper.SetConfigName(name)
			configFile = filepath.Join(configFile, name+"."+configExtension)
		} else {
			viper.SetConfigName(configName)
			configFile = filepath.Join(configFile, configName+"."+configExtension)
		}
	}

	viper.SetEnvPrefix(envPrefix)
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		klog.V(1).Infof("failed to read config file: %v", err)

		f.ConfigFile = configFile
	} else {
		f.ConfigFile = viper.ConfigFileUsed()
	}

	// initialize the factory

	// prefer an explicit GCTL_HOME env,
	// but fallback to the system-defined home directory
	home := os.Getenv(envGardenHomeDir)
	if len(home) == 0 {
		dir, err := homedir.Dir()
		cobra.CheckErr(err)

		home = filepath.Join(dir, gardenHomeFolder)
	}

	f.GardenHomeDirectory = home
}

func registerCompletionFuncForGlobalFlags(cmd *cobra.Command, f *util.FactoryImpl, ioStreams util.IOStreams) {
	utilruntime.Must(cmd.RegisterFlagCompletionFunc("garden", completionWrapper(f, ioStreams, gardenFlagCompletionFunc)))
	utilruntime.Must(cmd.RegisterFlagCompletionFunc("project", completionWrapper(f, ioStreams, projectFlagCompletionFunc)))
	utilruntime.Must(cmd.RegisterFlagCompletionFunc("seed", completionWrapper(f, ioStreams, seedFlagCompletionFunc)))
	utilruntime.Must(cmd.RegisterFlagCompletionFunc("shoot", completionWrapper(f, ioStreams, shootFlagCompletionFunc)))
}

type cobraCompletionFunc func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)
type cobraCompletionFuncWithError func(ctx context.Context, manager target.Manager) ([]string, error)

func completionWrapper(f *util.FactoryImpl, ioStreams util.IOStreams, completer cobraCompletionFuncWithError) cobraCompletionFunc {
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

// addKlogFlags adds flags from k8s.io/klog
func addKlogFlags(fs *pflag.FlagSet) {
	local := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	klog.InitFlags(local)

	local.VisitAll(func(fl *flag.Flag) {
		fs.AddGoFlag(fl)
	})
}
