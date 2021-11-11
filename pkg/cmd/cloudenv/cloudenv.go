/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package cloudenv

import (
	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
)

var (
	validShells = []string{"bash", "fish", "powershell", "zsh"}
	aliases     = []string{"cloudprovider-env", "provider-env"}
)

// NewCmdCloudEnv returns a new cloudenv command.
func NewCmdCloudEnv(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := NewCmdOptions(ioStreams)

	cmd := &cobra.Command{
		Use:       "cloud-env [bash | fish | powershell | zsh]",
		Short:     "Show the commands to configure cloudprovider CLI of the target cluster",
		Aliases:   aliases,
		ValidArgs: validShells,
		Args:      matchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs),
		RunE:      base.WrapRunE(o, f),
	}

	o.AddFlags(cmd.Flags())

	return cmd
}

func matchAll(checks ...cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		for _, check := range checks {
			if err := check(cmd, args); err != nil {
				return err
			}
		}

		return nil
	}
}
