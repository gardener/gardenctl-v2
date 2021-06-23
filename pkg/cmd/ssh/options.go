/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
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

	"github.com/gardener/gardener/pkg/utils"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	cryptossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Options is a struct to support ssh command
type Options struct {
	base.Options

	// Interactive can be used to toggle between gardenctl just
	// providing the bastion host while keeping it alive (non-interactive),
	// or gardenctl opening the SSH connection itself (interactive). For
	// interactive mode, a NodeName must be specified as well.
	Interactive bool

	// NodeName is the name of the Shoot cluster node that the user wants to
	// connect to. If this is left empty, gardenctl will only establish the
	// bastion host, but leave it up to the user to SSH themselves.
	NodeName string

	// CIDRs is a list of IP address ranges to be allowed for accessing the
	// created Bastion host. If not given, gardenctl will attempt to
	// auto-detect the user's IP and allow only it (i.e. use a /32 netmask).
	CIDRs []string

	// SSHPublicKeyFile is the full path to the file containing the user's
	// public SSH key. If not given, gardenctl will create a new temporary keypair.
	SSHPublicKeyFile string

	// SSHPrivateKeyFile is the full path to the file containing the user's
	// private SSH key. This is only set if no key was given and a temporary keypair
	// was generated. Otherwise gardenctl relies on the user's SSH agent.
	SSHPrivateKeyFile string

	// generatedSSHKeys is true if the public and private SSH keys have been generated
	// instead of being provided by the user. This will then be used for the cleanup.
	generatedSSHKeys bool

	// WaitTimeout is the maximum time to wait for a bastion to become ready.
	WaitTimeout time.Duration

	// KeepBastion will control whether or not gardenctl deletes the created
	// bastion once it exits. By default it deletes it, but we allow the user to
	// keep it for debugging purposes.
	KeepBastion bool
}

// NewOptions returns initialized Options
func NewOptions(ioStreams util.IOStreams) *Options {
	return &Options{
		Options: base.Options{
			IOStreams: ioStreams,
		},
		Interactive: true,
		WaitTimeout: 10 * time.Minute,
		KeepBastion: false,
	}
}

// Complete adapts from the command line args to the data required.
func (o *Options) Complete(f util.Factory, cmd *cobra.Command, args []string, stdout io.Writer) error {
	if len(o.CIDRs) == 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		publicIPs, err := f.PublicIPs(ctx)
		if err != nil {
			return fmt.Errorf("failed to determine your system's public IP addresses: %w", err)
		}

		cidrs := []string{}
		for _, ip := range publicIPs {
			cidrs = append(cidrs, ipToCIDR(ip))
		}

		name := "CIDR"
		if len(cidrs) != 1 {
			name = "CIDRs"
		}

		fmt.Fprintf(stdout, "Auto-detected your system's %s as %s\n", name, strings.Join(cidrs, ", "))

		o.CIDRs = cidrs
	}

	if len(o.SSHPublicKeyFile) == 0 {
		privateKeyFile, publicKeyFile, err := createSSHKeypair("", "")
		if err != nil {
			return fmt.Errorf("failed to generate SSH keypair: %w", err)
		}

		o.SSHPublicKeyFile = publicKeyFile
		o.SSHPrivateKeyFile = privateKeyFile
		o.generatedSSHKeys = true
	} else {
		count, err := countSSHAgentSigners()
		if err != nil {
			return fmt.Errorf("failed to check SSH agent status: %w", err)
		} else if count == 0 {
			return fmt.Errorf("a public key was provided, but no private key was found loaded into an SSH agent; this prevents gardenctl from validating the created bastion instance")
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

	// this is not yet normalized
	full := cidr.String()

	// normalize the CIDR
	// (e.g. turn "2001:0db8:0000:0000:0000:8a2e:0370:7334 /64" into "2001:db8::8a2e:370:7334/64")
	_, ipnet, _ := net.ParseCIDR(full)

	return ipnet.String()
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
			return fmt.Errorf("CIDR %q is invalid: %w", cidr, err)
		}
	}

	content, err := ioutil.ReadFile(o.SSHPublicKeyFile)
	if err != nil {
		return fmt.Errorf("invalid SSH public key file: %w", err)
	}

	if _, _, _, _, err := cryptossh.ParseAuthorizedKey(content); err != nil {
		return fmt.Errorf("invalid SSH public key file: %w", err)
	}

	return nil
}

func createSSHKeypair(tempDir string, keyName string) (string, string, error) {
	if keyName == "" {
		id, err := utils.GenerateRandomString(8)
		if err != nil {
			return "", "", fmt.Errorf("failed to create key name: %v", err)
		}

		keyName = fmt.Sprintf("gen_id_rsa_%s", strings.ToLower(id))
	}

	privateKey, err := createSSHPrivateKey()
	if err != nil {
		return "", "", fmt.Errorf("failed to create private key: %w", err)
	}

	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to create public key: %w", err)
	}

	if tempDir == "" {
		tempDir = os.TempDir()
	}

	privateKeyFile := filepath.Join(tempDir, keyName)
	if err := writeKeyFile(privateKeyFile, encodePrivateKey(privateKey)); err != nil {
		return "", "", fmt.Errorf("failed to write private key: %w", err)
	}

	publicKeyFile := filepath.Join(tempDir, fmt.Sprintf("%s.pub", keyName))
	if err := writeKeyFile(publicKeyFile, encodePublicKey(publicKey)); err != nil {
		return "", "", fmt.Errorf("failed to write public key: %w", err)
	}

	return privateKeyFile, publicKeyFile, nil
}

func createSSHPrivateKey() (*rsa.PrivateKey, error) {
	// Private Key generation
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	// Validate Private Key
	err = privateKey.Validate()
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}

func encodePrivateKey(privateKey *rsa.PrivateKey) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
}

func encodePublicKey(publicKey ssh.PublicKey) []byte {
	return ssh.MarshalAuthorizedKey(publicKey)
}

func writeKeyFile(filename string, content []byte) error {
	if err := ioutil.WriteFile(filename, content, 0600); err != nil {
		return fmt.Errorf("failed to write %q: %w", filename, err)
	}

	return nil
}

func countSSHAgentSigners() (int, error) {
	addr := os.Getenv("SSH_AUTH_SOCK")
	if len(addr) == 0 {
		return 0, nil
	}

	socket, err := net.Dial("unix", addr)
	if err != nil {
		return 0, fmt.Errorf("could not open SSH agent socket %q: %w", addr, err)
	}
	defer socket.Close()

	signers, err := agent.NewClient(socket).Signers()
	if err != nil {
		return 0, fmt.Errorf("error when retrieving signers from SSH agent: %w", err)
	}

	return len(signers), nil
}
