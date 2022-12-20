/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardenctl-v2/internal/util"
)

// AccessConfig is a struct that is embedded in the options of ssh related commands.
type AccessConfig struct {
	// CIDRs is a list of IP address ranges to be allowed for accessing the
	// created Bastion host. If not given, gardenctl will attempt to
	// auto-detect the user's IP and allow only it (i.e. use a /32 netmask).
	CIDRs []string

	// AutoDetected indicates if the public IPs of the user were automatically detected.
	// AutoDetected is false in case the CIDRs were provided via flags.
	AutoDetected bool
}

func (o *AccessConfig) Complete(f util.Factory, cmd *cobra.Command, args []string, ioStreams util.IOStreams) error {
	if len(o.CIDRs) == 0 {
		ctx, cancel := context.WithTimeout(f.Context(), 60*time.Second)
		defer cancel()

		publicIPs, err := f.PublicIPs(ctx)
		if err != nil {
			return fmt.Errorf("failed to determine your system's public IP addresses: %w", err)
		}

		var cidrs []string
		for _, ip := range publicIPs {
			cidrs = append(cidrs, ipToCIDR(ip))
		}

		name := "CIDR"
		if len(cidrs) != 1 {
			name = "CIDRs"
		}

		fmt.Fprintf(ioStreams.Out, "Auto-detected your system's %s as %s\n", name, strings.Join(cidrs, ", "))

		o.CIDRs = cidrs
		o.AutoDetected = true
	}

	return nil
}

func (o *AccessConfig) Validate() error {
	if len(o.CIDRs) == 0 {
		return errors.New("must at least specify a single CIDR to allow access to the bastion")
	}

	for _, cidr := range o.CIDRs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("CIDR %q is invalid: %w", cidr, err)
		}
	}

	return nil
}

func (o *AccessConfig) AddFlags(flags *pflag.FlagSet) {
	flags.StringArrayVar(&o.CIDRs, "cidr", nil, "CIDRs to allow access to the bastion host; if not given, your system's public IPs (v4 and v6) are auto-detected.")
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

func RegisterCompletionFuncsForAccessConfigFlags(cmd *cobra.Command, factory util.Factory, ioStreams util.IOStreams, flags *pflag.FlagSet) {
	utilruntime.Must(cmd.RegisterFlagCompletionFunc("cidr", completionWrapper(factory, ioStreams, cidrFlagCompletionFunc)))
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
