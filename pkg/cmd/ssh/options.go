/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	cryptossh "golang.org/x/crypto/ssh"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// Options is a struct to support target command
type Options struct {
	base.Options

	// Interactive can be used to toggle between gardenctl just
	// providing the bastion host while keeping it alive (non-interactive),
	// or gardenctl opening the SSH connection itself (interactive). For
	// interactive mode, a NodeName must be specified as well.
	Interactive bool

	// NodeName is the name of the Shoot cluster node that the user wants to
	// connect to. If this is left empty, grdenctl will only establish the
	// bastion host, but leave it up to the user to SSH themselves.
	NodeName string

	// CIDRs is a list of IP address ranges to allowed for accessing the
	// created Bastion host. If not given, gardenctl will attempt to
	// auto-detect the user's IP and allow only it (i.e. use a /32 netmask).
	CIDRs []string

	// SSHPublicKeyFile is the full path to the file containing the user's
	// public SSH key. If not given, gardenctl will try ~/.ssh/id_rsa.pub and then
	// ~/.ssh/id_ed25519.pub.
	SSHPublicKeyFile string

	// WaitTimeout is the maximum time to wait for a bastion to become ready.
	WaitTimeout time.Duration
}

// NewOptions returns initialized Options
func NewOptions(ioStreams genericclioptions.IOStreams) *Options {
	return &Options{
		Options: base.Options{
			IOStreams: ioStreams,
		},
		Interactive: true,
		WaitTimeout: 10 * time.Minute,
	}
}

// Complete adapts from the command line args to the data required.
func (o *Options) Complete(f util.Factory, cmd *cobra.Command, args []string, stdout io.Writer) error {
	if len(o.CIDRs) == 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		publicIP, err := f.PublicIP(ctx)
		if err != nil {
			return fmt.Errorf("failed to determine your system's public IP address: %v", err)
		}

		fmt.Fprintf(stdout, "Auto-detected public IP as %s\n", publicIP)

		o.CIDRs = append(o.CIDRs, ipToCIDR(publicIP))
	}

	if len(o.SSHPublicKeyFile) == 0 {
		if home, err := homedir.Dir(); err == nil {
			for _, filename := range []string{"id_rsa.pub", "id_ed25519.pub"} {
				fullFilename := filepath.Join(home, ".ssh", filename)

				if _, err := os.Stat(fullFilename); err == nil {
					fmt.Fprintf(stdout, "Using SSH public key %s\n", fullFilename)
					o.SSHPublicKeyFile = fullFilename
					break
				}
			}
		}
	}

	if len(args) > 0 {
		o.NodeName = strings.TrimSpace(args[0])
	}

	return nil
}

func ipToCIDR(address string) string {
	ip := net.ParseIP(address)

	var mask net.IPMask
	if ip.To4() != nil {
		mask = net.CIDRMask(32, 32) // use a /32 net for IPv4
	} else {
		mask = net.CIDRMask(64, 128) // use a /64 net for IPv6
	}

	cidr := net.IPNet{
		IP:   ip,
		Mask: mask,
	}

	return cidr.String()
}

// Validate validates the provided options
func (o *Options) Validate() error {
	if o.WaitTimeout == 0 {
		return errors.New("the maximum wait duration must be non-zero")
	}

	if len(o.CIDRs) == 0 {
		return errors.New("must at least specify a single CIDR to allow access to the bastion")
	}

	for _, cidr := range o.CIDRs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("CIDR %q is invalid: %v", cidr, err)
		}
	}

	if len(o.SSHPublicKeyFile) == 0 {
		return errors.New("no SSH key found, please specify public key file explicitly")
	}

	content, err := ioutil.ReadFile(o.SSHPublicKeyFile)
	if err != nil {
		return fmt.Errorf("invalid SSH key file: %v", err)
	}

	if _, _, _, _, err := cryptossh.ParseAuthorizedKey(content); err != nil {
		return fmt.Errorf("invalid SSH key file: %v", err)
	}

	return nil
}
