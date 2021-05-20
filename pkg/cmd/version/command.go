/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package version

import (
	"encoding/json"
	"fmt"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"k8s.io/component-base/version"
)

// NewCommand returns a new version command.
func NewCommand(f util.Factory, o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the gardenctl version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(f, cmd, args); err != nil {
				return fmt.Errorf("failed to complete command options: %w", err)
			}
			if err := o.Validate(); err != nil {
				return err
			}

			return runCommand(o)
		},
	}

	cmd.Flags().BoolVar(&o.Short, "short", o.Short, "If true, print just the version number.")
	cmd.Flags().StringVarP(&o.Output, "output", "o", o.Output, "One of 'yaml' or 'json'.")

	return cmd
}

func runCommand(opt *Options) error {
	versionInfo := version.Get()

	switch opt.Output {
	case "":
		if opt.Short {
			fmt.Fprintf(opt.IOStreams.Out, "Version: %s\n", versionInfo.GitVersion)
		} else {
			fmt.Fprintf(opt.IOStreams.Out, "Version: %s\n", fmt.Sprintf("%#v", versionInfo))
		}

	case "yaml":
		marshalled, err := yaml.Marshal(&versionInfo)
		if err != nil {
			return err
		}

		fmt.Fprintln(opt.IOStreams.Out, string(marshalled))

	case "json":
		marshalled, err := json.MarshalIndent(&versionInfo, "", "  ")
		if err != nil {
			return err
		}

		fmt.Fprintln(opt.IOStreams.Out, string(marshalled))

	default:
		// There is a bug in the program if we hit this case.
		// However, we follow a policy of never panicking.
		return fmt.Errorf("options were not validated: --output=%q should have been rejected", opt.Output)
	}

	return nil
}
