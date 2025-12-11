/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh

import (
	"strings"

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
		Short: "Establish an SSH connection to a node of a Shoot cluster",
		Long: `Establish an SSH connection to a node of a Shoot cluster by specifying its name.

A bastion is created to access the node and is automatically cleaned up afterwards.

If a node name is not provided, gardenctl will display the hostnames/IPs of the Shoot worker nodes and the corresponding SSH command.
To connect to a desired node, copy the printed SSH command, replace the target hostname accordingly, and execute the command.`,
		Example: `# Establish an SSH connection to a specific Shoot cluster node
gardenctl ssh my-shoot-node-1

# Establish an SSH connection with custom CIDRs to allow access to the bastion host
gardenctl ssh my-shoot-node-1 --cidr 10.1.2.3/32

# Establish an SSH connection to any Shoot cluster node
# Copy the printed SSH command, replace the 'IP_OR_HOSTNAME' placeholder for the target hostname/IP, and execute the command to connect to the desired node
gardenctl ssh

# Create the bastion and output the connection information in JSON format
gardenctl ssh --no-keepalive --keep-bastion --interactive=false --output json

# Reuse a previously created bastion
gardenctl ssh --keep-bastion --bastion-name cli-xxxxxxxx --public-key-file /path/to/ssh/key.pub --private-key-file /path/to/ssh/key
`,
		Args: cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			ctx := f.Context()
			logger := klog.FromContext(ctx)

			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			manager, err := f.Manager()
			if err != nil {
				logger.Error(err, "could not get manager from factory")
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			nodeNames, err := getNodeNamesFromMachinesOrNodes(ctx, manager)
			if err != nil {
				logger.Error(err, "could not get node names from shoot")
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			var completions []string

			for _, nodeName := range nodeNames {
				if strings.HasPrefix(nodeName, toComplete) {
					completions = append(completions, nodeName)
				}
			}

			return completions, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: base.WrapRunE(o, f),
	}

	o.AddFlags(cmd.Flags())
	o.RegisterCompletionsForOutputFlag(cmd)
	o.RegisterCompletionFuncsForStrictHostKeyCheckings(cmd)
	o.RegisterCompletionFuncForShell(cmd)

	o.AccessConfig.AddFlags(cmd.Flags())
	RegisterCompletionFuncsForAccessConfigFlags(cmd, f)

	f.TargetFlags().AddFlags(cmd.Flags())
	flags.RegisterCompletionFuncsForTargetFlags(cmd, f, o.IOStreams, cmd.Flags())

	return cmd
}
