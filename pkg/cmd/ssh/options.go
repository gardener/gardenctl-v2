/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
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

	// CIDRs is a list of IP address ranges to allowed for accessing the
	// created Bastion host. If not given, gardenctl will attempt to
	// auto-detect the user's IP and allow only it (i.e. use a /32 netmask).
	CIDRs []string

	// SSHPublicKeyFile is the full path to the file containing the user's
	// public SSH key. If not given, gardenctl will try ~/.ssh/id_rsa.pub and then
	// ~/.ssh/id_ed25519.pub.
	SSHPublicKeyFile string
}

// NewOptions returns initialized Options
func NewOptions(ioStreams genericclioptions.IOStreams) *Options {
	return &Options{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
}

// Complete adapts from the command line args to the data required.
func (o *Options) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	if len(o.CIDRs) == 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		publicIP, err := getPublicIP(ctx)
		if err != nil {
			return fmt.Errorf("failed to determine your system's public IP address: %v", err)
		}

		// TODO: This string handling breaks with IPv6
		o.CIDRs = append(o.CIDRs, fmt.Sprintf("%s/32", publicIP))
	}

	if len(o.SSHPublicKeyFile) == 0 {
		if home, err := homedir.Dir(); err == nil {
			for _, filename := range []string{"id_rsa.pub", "id_ed25519.pub"} {
				fullFilename := filepath.Join(home, ".ssh", filename)
				fmt.Println(fullFilename)

				if _, err := os.Stat(fullFilename); err == nil {
					o.SSHPublicKeyFile = fullFilename
					break
				}
			}
		}
	}

	return nil
}

// Validate validates the provided options
func (o *Options) Validate() error {
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
