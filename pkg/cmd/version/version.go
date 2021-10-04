/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package version

import (
	"fmt"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"

	"github.com/spf13/cobra"
	"k8s.io/component-base/version"
)

// NewCmdVersion returns a new version command.
func NewCmdVersion(f util.Factory, o *VersionOptions) *cobra.Command {
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

			return runCmdVersion(o)
		},
	}

	cmd.Flags().BoolVar(&o.Short, "short", o.Short, "If true, print just the version number.")
	o.AddOutputFlags(cmd)

	return cmd
}

func runCmdVersion(opt *VersionOptions) error {
	versionInfo := version.Get()

	if opt.Output == "" {
		if opt.Short {
			fmt.Fprintf(opt.IOStreams.Out, "Version: %s\n", versionInfo.GitVersion)
		} else {
			fmt.Fprintf(opt.IOStreams.Out, "Version: %s\n", fmt.Sprintf("%#v", versionInfo))
		}
	} else {
		if opt.Short {
			return opt.PrintObject(versionInfo.GitVersion)
		}
		return opt.PrintObject(versionInfo)
	}

	return nil
}

// VersionOptions is a struct to support version command
// nolint
type VersionOptions struct {
	base.Options

	// Short indicates if just the version number should be printed
	Short bool
}

// NewVersionOptions returns initialized VersionOptions
func NewVersionOptions(ioStreams util.IOStreams) *VersionOptions {
	return &VersionOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
}

// Complete adapts from the command line args to the data required.
func (o *VersionOptions) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	return nil
}
