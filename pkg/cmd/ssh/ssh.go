/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
)

// NewCmdSSH returns a new ssh command.
func NewCmdSSH(f util.Factory, o *SSHOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh [NODE_NAME]",
		Short: "Establish an SSH connection to a Shoot cluster's node",
		Args:  cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			nodeNames, err := getNodeNamesFromShoot(f, toComplete)
			if err != nil {
				fmt.Fprintln(o.IOStreams.ErrOut, err.Error())
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			return nodeNames, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: base.WrapRunE(o, f),
	}

	cmd.Flags().BoolVar(&o.Interactive, "interactive", o.Interactive, "Open an SSH connection instead of just providing the bastion host (only if NODE_NAME is provided).")
	cmd.Flags().StringArrayVar(&o.CIDRs, "cidr", nil, "CIDRs to allow access to the bastion host; if not given, your system's public IPs (v4 and v6) are auto-detected.")
	cmd.Flags().StringVar(&o.SSHPublicKeyFile, "public-key-file", "", "Path to the file that contains a public SSH key. If not given, a temporary keypair will be generated.")
	cmd.Flags().DurationVar(&o.WaitTimeout, "wait-timeout", o.WaitTimeout, "Maximum duration to wait for the bastion to become available.")
	cmd.Flags().BoolVar(&o.KeepBastion, "keep-bastion", o.KeepBastion, "Do not delete immediately when gardenctl exits (Bastions will be garbage-collected after some time)")

	return cmd
}
