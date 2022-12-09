package ssh

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
)

// BaseOptions is a struct used by all ssh related commands
type BaseOptions struct {
	base.Options

	// CIDRs is a list of IP address ranges to be allowed for accessing the
	// created Bastion host. If not given, gardenctl will attempt to
	// auto-detect the user's IP and allow only it (i.e. use a /32 netmask).
	CIDRs []string

	// AutoDetected indicates if the public IPs of the user were automatically detected.
	// AutoDetected is false in case the CIDRs were provided via flags.
	AutoDetected bool
}

func (o *BaseOptions) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
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

		fmt.Fprintf(o.IOStreams.Out, "Auto-detected your system's %s as %s\n", name, strings.Join(cidrs, ", "))

		o.CIDRs = cidrs
		o.AutoDetected = true
	}

	return nil
}

func (o *BaseOptions) Validate() error {
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
