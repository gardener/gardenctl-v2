/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh

import (
	"fmt"
	"net"

	"github.com/spf13/cobra"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

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

	manager, err := f.Manager()
	utilruntime.Must(err)
	manager.TargetFlags().AddFlags(cmd.Flags())
	flags.RegisterCompletionFuncsForTargetFlags(cmd, f, o.IOStreams, cmd.Flags())

	registerCompletionFuncForFlags(cmd, f, o.IOStreams)

	return cmd
}

func registerCompletionFuncForFlags(cmd *cobra.Command, f util.Factory, ioStreams util.IOStreams) {
	utilruntime.Must(cmd.RegisterFlagCompletionFunc("cidr", completionWrapper(f, ioStreams, cidrFlagCompletionFunc)))
}

type (
	cobraCompletionFunc          func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)
	cobraCompletionFuncWithError func(f util.Factory) ([]string, error)
)

func completionWrapper(f util.Factory, ioStreams util.IOStreams, completer cobraCompletionFuncWithError) cobraCompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		result, err := completer(f)
		if err != nil {
			fmt.Fprintf(ioStreams.ErrOut, "%v\n", err)
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		return util.FilterStringsByPrefix(toComplete, result), cobra.ShellCompDirectiveNoFileComp
	}
}

func cidrFlagCompletionFunc(f util.Factory) ([]string, error) {
	var addresses []string

	ctx := f.Context()

	publicIPs, err := f.PublicIPs(ctx)
	if err != nil {
		return nil, err
	}

	for _, ip := range publicIPs {
		cidr := ipToCIDR(ip)
		addresses = append(addresses, fmt.Sprintf("%s\t<public>", cidr))
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	includeFlags := net.FlagUp
	excludeFlags := net.FlagLoopback

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}

		if is(iface, includeFlags) && isNot(iface, excludeFlags) {
			for _, addr := range addrs {
				addressComp := fmt.Sprintf("%s\t%s", addr.String(), iface.Name)
				addresses = append(addresses, addressComp)
			}
		}
	}

	return addresses, nil
}

func is(i net.Interface, flags net.Flags) bool {
	return i.Flags&flags != 0
}

func isNot(i net.Interface, flags net.Flags) bool {
	return i.Flags&flags == 0
}
