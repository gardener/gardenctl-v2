/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package kubectlenv

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/env"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

type options struct {
	base.Options

	// Unset resets environment variables and configuration of the cloudprovider CLI for your shell.
	Unset bool
	// Shell to configure.
	Shell string
	// GardenDir is the configuration directory of gardenctl.
	GardenDir string
	// SessionDir is the session directory of gardenctl.
	SessionDir string
	// CmdPath is the path of the called command.
	CmdPath string
	// Target is the target used when executing the command
	Target target.Target
	// Template is the script template
	Template env.Template
	// Symlink indicates if KUBECONFIG environment variable should point to the session stable symlink
	Symlink bool
}

// Complete adapts from the command line args to the data required.
func (o *options) Complete(f util.Factory, cmd *cobra.Command, _ []string) error {
	o.Shell = cmd.Name()
	o.CmdPath = cmd.Parent().CommandPath()
	o.GardenDir = f.GardenHomeDir()

	tmpl, err := env.NewTemplate(o.Shell, "helpers")
	if err != nil {
		return err
	}

	o.Template = tmpl

	filename := filepath.Join(o.GardenDir, "templates", "kubernetes.tmpl")
	if err := o.Template.ParseFiles(filename); err != nil {
		return err
	}

	manager, err := f.Manager()
	if err != nil {
		return err
	}

	o.Symlink = manager.Configuration().SymlinkTargetKubeconfig()
	o.SessionDir = manager.SessionDir()

	return nil
}

// Validate validates the provided command options.
func (o *options) Validate() error {
	if o.Shell == "" {
		return pflag.ErrHelp
	}

	s := env.Shell(o.Shell)

	return s.Validate()
}

// AddFlags binds the command options to a given flagset.
func (o *options) AddFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&o.Unset, "unset", "u", o.Unset, fmt.Sprintf("Generate the script to unset the KUBECONFIG environment variable for %s", o.Shell))
}

// Run does the actual work of the command.
func (o *options) Run(f util.Factory) error {
	ctx := f.Context()

	manager, err := f.Manager()
	if err != nil {
		return err
	}

	o.Target, err = manager.CurrentTarget()
	if err != nil {
		return err
	}

	if !o.Symlink && o.Target.GardenName() == "" {
		return target.ErrNoGardenTargeted
	}

	data := map[string]interface{}{
		"__meta": generateMetadata(o),
	}

	if !o.Unset {
		var filename string

		if o.Symlink {
			filename = filepath.Join(o.SessionDir, "kubeconfig.yaml")

			if !o.Target.IsEmpty() {
				_, err := os.Lstat(filename)
				if os.IsNotExist(err) {
					return fmt.Errorf("symlink to targeted cluster does not exist: %w", err)
				}
			}
		} else {
			config, err := manager.ClientConfig(ctx, o.Target)
			if err != nil {
				return err
			}

			filename, err = manager.WriteClientConfig(config)
			if err != nil {
				return err
			}
		}

		data["filename"] = filename
	}

	return o.Template.ExecuteTemplate(o.IOStreams.Out, o.Shell, data)
}

func generateMetadata(o *options) map[string]interface{} {
	metadata := make(map[string]interface{})
	metadata["unset"] = o.Unset
	metadata["shell"] = o.Shell
	metadata["commandPath"] = o.CmdPath
	metadata["cli"] = "kubectl"
	metadata["prompt"] = env.Shell(o.Shell).Prompt(runtime.GOOS)
	metadata["targetFlags"] = getTargetFlags(o.Target)

	return metadata
}

func getTargetFlags(t target.Target) string {
	if t.ProjectName() != "" {
		return fmt.Sprintf("--garden %s --project %s --shoot %s", t.GardenName(), t.ProjectName(), t.ShootName())
	}

	return fmt.Sprintf("--garden %s --seed %s --shoot %s", t.GardenName(), t.SeedName(), t.ShootName())
}
