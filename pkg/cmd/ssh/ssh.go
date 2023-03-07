/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh

import (
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/flags"
)

// NewCmdSSH returns a new ssh command.
func NewCmdSSH(f util.Factory, o *SSHOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh [NODE_NAME]",
		Short: "Establish an SSH connection to a Shoot cluster's node",
		Args:  cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			ctx := f.Context()
			logger := klog.FromContext(ctx)

			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			nodeNames, err := getNodeNamesFromShoot(f, toComplete)
			if err != nil {
				logger.Error(err, "could not get node names from shoot")
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			return nodeNames, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: base.WrapRunE(o, f),
	}

	o.AddFlags(cmd.Flags())
	o.AccessConfig.AddFlags(cmd.Flags())
	RegisterCompletionFuncsForAccessConfigFlags(cmd, f)

	f.TargetFlags().AddFlags(cmd.Flags())
	flags.RegisterCompletionFuncsForTargetFlags(cmd, f, o.IOStreams, cmd.Flags())

	return cmd
}
