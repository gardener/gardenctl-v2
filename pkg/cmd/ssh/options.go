/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

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
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	corev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	corev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/secrets"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/ac"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

const (
	// SSHBastionUsername is the system username on the bastion host.
	SSHBastionUsername = "gardener"
	// DefaultUsername is the default Shoot cluster node ssh login username.
	DefaultUsername = "gardener"
	// SSHPort is the TCP port on a bastion instance that allows incoming SSH.
	SSHPort = 22
)

// wrappers used for unit tests only.
var (
	// keepAliveInterval is the interval in which bastions should be given the
	// keep-alive annotation to prolong their lifetime.
	keepAliveInterval      = 3 * time.Minute
	keepAliveIntervalMutex sync.RWMutex

	// pollBastionStatusInterval is the time in-between status checks on the bastion object.
	pollBastionStatusInterval = 5 * time.Second

	// tempFileCreator creates and opens a temporary file.
	tempFileCreator = func() (*os.File, error) {
		return os.CreateTemp(os.TempDir(), "gctlv2*")
	}

	// bastionAvailabilityChecker returns nil if the given hostname allows incoming
	// connections on the SSHPort and has a public key configured that matches the
	// given private key.
	bastionAvailabilityChecker = func(hostname string, port string, privateKey []byte) error {
		authMethods := []ssh.AuthMethod{}

		if len(privateKey) > 0 {
			signer, err := ssh.ParsePrivateKey(privateKey)
			if err != nil {
				return fmt.Errorf("invalid private SSH key: %w", err)
			}

			authMethods = append(authMethods, ssh.PublicKeys(signer))
		} else if addr := os.Getenv("SSH_AUTH_SOCK"); len(addr) > 0 {
			socket, dialErr := net.Dial("unix", addr)
			if dialErr != nil {
				return fmt.Errorf("could not open SSH agent socket %q: %w", addr, dialErr)
			}
			defer socket.Close()

			signers, signersErr := agent.NewClient(socket).Signers()
			if signersErr != nil {
				return fmt.Errorf("error when creating signer for SSH agent: %w", signersErr)
			}

			authMethods = append(authMethods, ssh.PublicKeys(signers...))
		} else {
			return errors.New("neither private key nor the environment variable SSH_AUTH_SOCK are defined, cannot connect to bastion")
		}

		client, err := ssh.Dial("tcp", net.JoinHostPort(hostname, port), &ssh.ClientConfig{
			User:            SSHBastionUsername,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Auth:            authMethods,
			Timeout:         10 * time.Second,
		})

		if client != nil {
			client.Close()
		}

		return err
	}

	// bastionNameProvider generates the name for a new bastion.
	bastionNameProvider = func() (string, error) {
		bastionID, err := utils.GenerateRandomString(8)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("cli-%s", strings.ToLower(bastionID)), nil
	}

	// createSignalChannel returns a channel which receives OS signals.
	createSignalChannel = func() chan os.Signal {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt)

		return signalChan
	}

	// execCommand executes the given command, using the in/out streams
	// from the SSHOptions. The function returns an error if the command
	// fails.
	execCommand = func(ctx context.Context, command string, args []string, ioStreams util.IOStreams) error {
		cmd := exec.CommandContext(ctx, command, args...)
		cmd.Stdout = ioStreams.Out
		cmd.Stdin = ioStreams.In
		cmd.Stderr = ioStreams.ErrOut

		return cmd.Run()
	}

	// waitForSignal informs the user how to stop gardenctl and keeps the
	// bastion alive until gardenctl exits.
	waitForSignal = func(ctx context.Context, o *SSHOptions, signalChan <-chan struct{}) {
		if o.Output == "" {
			fmt.Fprintln(o.IOStreams.Out, "> Press Ctrl-C to stop gardenctl, after which the bastion will be removed.")
		}

		// keep the bastion alive until gardenctl exits
		<-signalChan
	}
)

// SSHOptions is a struct to support ssh command.
type SSHOptions struct {
	base.Options
	AccessConfig
	// Interactive can be used to toggle between gardenctl just
	// providing the bastion host while keeping it alive (non-interactive),
	// or gardenctl opening the SSH connection itself (interactive). For
	// interactive mode, a NodeName must be specified as well.
	Interactive bool

	// BastionName is the name of the bastion. If not provided, a unique name will be
	// automatically generated.
	BastionName string

	// BastionHost overrides the hostname or IP address of the Bastion used for the SSH command.
	// If not provided, the address will be determined from .status.ingress.ip or
	// status.ingress.hostname of the Bastion.
	BastionHost string

	// BastionPort is the SSH port for the bastion host
	BastionPort string

	// BastionUserKnownHostsFiles is a list of custom known hosts files for the SSH connection to the bastion.
	BastionUserKnownHostsFiles []string

	// NodeName is the name of the Shoot cluster node that the user wants to
	// connect to. If this is left empty, gardenctl will only establish the
	// bastion host, but leave it up to the user to SSH themselves.
	NodeName string

	// User is the name of the Shoot cluster node ssh login username
	User string

	// SSHPublicKeyFile is the full path to the file containing the user's
	// public SSH key. If not given, gardenctl will create a new temporary keypair.
	SSHPublicKeyFile PublicKeyFile

	// SSHPrivateKeyFile is the full path to the file containing the user's
	// private SSH key. If not set, gardenctl relies on the user's SSH agent.
	SSHPrivateKeyFile PrivateKeyFile

	// GeneratedSSHKeys is true if the public and private SSH keys have been generated
	// instead of being provided by the user. This will then be used for the cleanup.
	GeneratedSSHKeys bool

	// WaitTimeout is the maximum time to wait for a bastion to become ready.
	WaitTimeout time.Duration

	// KeepBastion will control whether or not gardenctl deletes the created
	// bastion once it exits. By default it deletes it, but we allow the user to
	// keep it for debugging purposes.
	KeepBastion bool

	// SkipAvailabilityCheck determines whether to check for the availability of
	// the bastion host.
	SkipAvailabilityCheck bool

	// NoKeepalive controls if the command should exit after the bastion becomes available.
	// If this option is true, no SSH connection will be established and the bastion will
	// not be kept alive after it became available.
	// This option can only be used if KeepBastion is set to true and Interactive is set to false.
	NoKeepalive bool

	// ConfirmAccessRestriction, when set to true, implies the user understands the access restrictions for the targeted shoot.
	// In this case, the access restriction banner is displayed without further confirmation.
	ConfirmAccessRestriction bool
}

// NewSSHOptions returns initialized SSHOptions.
func NewSSHOptions(ioStreams util.IOStreams) *SSHOptions {
	return &SSHOptions{
		AccessConfig: AccessConfig{},
		Options: base.Options{
			IOStreams: ioStreams,
		},
		Interactive:           true,
		WaitTimeout:           10 * time.Minute,
		KeepBastion:           false,
		SkipAvailabilityCheck: false,
		NoKeepalive:           false,
		BastionPort:           strconv.Itoa(SSHPort),
		User:                  DefaultUsername,
	}
}

func (o *SSHOptions) AddFlags(flagSet *pflag.FlagSet) {
	flagSet.BoolVar(&o.Interactive, "interactive", o.Interactive, "Open an SSH connection instead of just providing the bastion host (only if NODE_NAME is provided).")
	flagSet.Var(&o.SSHPublicKeyFile, "public-key-file", "Path to the file that contains a public SSH key. If not given, a temporary keypair will be generated.")
	flagSet.Var(&o.SSHPrivateKeyFile, "private-key-file", "Path to the file that contains a private SSH key. Must be provided alongside the --public-key-file flag if you want to use a custom keypair. If not provided, gardenctl will either generate a temporary keypair or rely on the user's SSH agent for an available private key.")
	flagSet.DurationVar(&o.WaitTimeout, "wait-timeout", o.WaitTimeout, "Maximum duration to wait for the bastion to become available.")
	flagSet.BoolVar(&o.KeepBastion, "keep-bastion", o.KeepBastion, "Do not delete immediately when gardenctl exits (Bastions will be garbage-collected after some time)")
	flagSet.BoolVar(&o.SkipAvailabilityCheck, "skip-availability-check", o.SkipAvailabilityCheck, "Skip checking for SSH bastion host availability.")
	flagSet.BoolVar(&o.NoKeepalive, "no-keepalive", o.NoKeepalive, "Exit after the bastion host became available without keeping the bastion alive or establishing an SSH connection. Note that this flag requires the flags --interactive=false and --keep-bastion to be set")
	flagSet.StringVar(&o.BastionName, "bastion-name", o.BastionName, "Name of the bastion. If a bastion with this name doesn't exist, it will be created. If it does exist, the provided public SSH key must match the one used during the bastion's creation.")
	flagSet.StringVar(&o.BastionHost, "bastion-host", o.BastionHost, "Override the hostname or IP address of the bastion used for the SSH client command. If not provided, the address will be automatically determined.")
	flagSet.StringVar(&o.BastionPort, "bastion-port", o.BastionPort, "SSH port of the bastion used for the SSH client command. Defaults to port 22")
	flagSet.StringSliceVar(&o.BastionUserKnownHostsFiles, "bastion-user-known-hosts-file", o.BastionUserKnownHostsFiles, "Path to a custom known hosts file for the SSH connection to the bastion. This file is used to verify the public keys of remote hosts when establishing a secure connection.")
	flagSet.BoolVarP(&o.ConfirmAccessRestriction, "confirm-access-restriction", "y", o.ConfirmAccessRestriction, "Bypasses the need for confirmation of any access restrictions. Set this flag only if you are fully aware of the access restrictions.")
	flagSet.StringVar(&o.User, "user", o.User, "user is the name of the Shoot cluster node ssh login username.")
	o.Options.AddFlags(flagSet)
}

// Complete adapts from the command line args to the data required.
func (o *SSHOptions) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	ctx := f.Context()
	logger := klog.FromContext(ctx)

	if err := o.AccessConfig.Complete(f, cmd, args); err != nil {
		return err
	}

	if len(o.SSHPublicKeyFile) == 0 {
		privateKeyFile, publicKeyFile, err := createSSHKeypair("", "")
		if err != nil {
			return fmt.Errorf("failed to generate SSH keypair: %w", err)
		}

		o.SSHPublicKeyFile = publicKeyFile
		o.SSHPrivateKeyFile = privateKeyFile
		o.GeneratedSSHKeys = true
	}

	if len(o.SSHPrivateKeyFile) == 0 {
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

	if o.NodeName == "" && o.Interactive {
		logger.V(4).Info("no node name given, switching to non-interactive mode")

		o.Interactive = false
	}

	if o.BastionName == "" {
		name, err := bastionNameProvider()
		if err != nil {
			return fmt.Errorf("failed to create bastion name: %w", err)
		}

		o.BastionName = name
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

// Validate validates the provided SSHOptions.
func (o *SSHOptions) Validate() error {
	if err := o.Options.Validate(); err != nil {
		return err
	}

	if err := o.AccessConfig.Validate(); err != nil {
		return err
	}

	if o.WaitTimeout == 0 {
		return errors.New("the maximum wait duration must be non-zero")
	}

	if o.NoKeepalive {
		if o.Interactive {
			return errors.New("set --interactive=false when disabling keepalive")
		}

		if !o.KeepBastion {
			return errors.New("set --keep-bastion when disabling keepalive")
		}
	}

	if o.Output != "" {
		if o.Interactive {
			return errors.New("set --interactive=false when using the output flag")
		}
	}

	if o.User == "" {
		return errors.New("user must not be empty")
	}

	content, err := os.ReadFile(o.SSHPublicKeyFile.String())
	if err != nil {
		return fmt.Errorf("invalid SSH public key file: %w", err)
	}

	if _, _, _, _, err := ssh.ParseAuthorizedKey(content); err != nil {
		return fmt.Errorf("invalid SSH public key file: %w", err)
	}

	return nil
}

func createSSHKeypair(tempDir string, keyName string) (PrivateKeyFile, PublicKeyFile, error) {
	if keyName == "" {
		id, err := utils.GenerateRandomString(8)
		if err != nil {
			return "", "", fmt.Errorf("failed to create key name: %w", err)
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

	sshPrivateKeyFile := PrivateKeyFile(filepath.Join(tempDir, keyName))
	if err := writeKeyFile(sshPrivateKeyFile.String(), encodePrivateKey(privateKey)); err != nil {
		return "", "", fmt.Errorf("failed to write private key: %w", err)
	}

	sshPublicKeyFile := PublicKeyFile(filepath.Join(tempDir, fmt.Sprintf("%s.pub", keyName)))
	if err := writeKeyFile(sshPublicKeyFile.String(), encodePublicKey(publicKey)); err != nil {
		return "", "", fmt.Errorf("failed to write public key: %w", err)
	}

	return sshPrivateKeyFile, sshPublicKeyFile, nil
}

func createSSHPrivateKey() (*rsa.PrivateKey, error) {
	// Private Key generation
	privateKey, err := rsa.GenerateKey(rand.Reader, 3072)
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
	if err := os.WriteFile(filename, content, 0o600); err != nil {
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

func (o *SSHOptions) Run(f util.Factory) error {
	manager, err := f.Manager()
	if err != nil {
		return err
	}

	ctx := f.Context()
	logger := klog.FromContext(ctx)

	// currentTarget is the target used for the run method
	currentTarget, err := manager.CurrentTarget()
	if err != nil {
		return err
	}

	// create client for the garden cluster
	gardenClient, err := manager.GardenClient(currentTarget.GardenName())
	if err != nil {
		return err
	}

	if currentTarget.ShootName() == "" && currentTarget.SeedName() != "" {
		shoot, err := gardenClient.GetShootOfManagedSeed(ctx, currentTarget.SeedName())
		if err != nil {
			if apierrors.IsNotFound(err) {
				return fmt.Errorf("cannot ssh to non-managed seeds: %w", err)
			}

			return err
		}

		logger.V(1).Info("using referred shoot of managed seed",
			"shoot", klog.ObjectRef{
				Namespace: "garden",
				Name:      shoot.Name,
			},
			"seed", currentTarget.SeedName())

		currentTarget = currentTarget.WithProjectName("garden").WithShootName(shoot.Name)
	}

	if currentTarget.ShootName() == "" {
		return target.ErrNoShootTargeted
	}

	printTargetInformation(logger, currentTarget)

	// fetch targeted shoot (ctx is cancellable to stop the keep alive goroutine later)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	shoot, err := gardenClient.FindShoot(ctx, currentTarget.AsListOption())
	if err != nil {
		return err
	}

	// check access restrictions
	ok, err := o.checkAccessRestrictions(manager.Configuration(), currentTarget.GardenName(), f.TargetFlags(), shoot)
	if err != nil {
		return err
	} else if !ok {
		return nil // abort
	}

	workersSettings := shoot.Spec.Provider.WorkersSettings
	if workersSettings != nil && workersSettings.SSHAccess != nil && !workersSettings.SSHAccess.Enabled {
		return errors.New("node SSH access disabled, SSH not allowed")
	}

	// fetch the SSH key(s) for the shoot nodes
	nodePrivateKeys, err := getShootNodePrivateKeys(ctx, gardenClient.RuntimeClient(), shoot)
	if err != nil {
		return err
	}

	// save the keys into temporary files that we try to clean up when exiting
	var nodePrivateKeyFiles []PrivateKeyFile

	for _, pk := range nodePrivateKeys {
		filename, err := writeToTemporaryFile(pk)
		if err != nil {
			return err
		}

		nodePrivateKeyFiles = append(nodePrivateKeyFiles, PrivateKeyFile(filename))
	}

	shootClient, err := manager.ShootClient(ctx, currentTarget)
	if err != nil {
		return err
	}

	var nodeHostname string

	if o.NodeName != "" {
		node, err := getShootNode(ctx, o, shootClient)
		if err == nil { //nolint:gocritic // rewrite if-else to switch statement does not make sense as anonymous switch statements should never be cuddled
			nodeHostname, err = getNodeHostname(node)
			if err != nil {
				return err
			}
		} else if apierrors.IsNotFound(err) {
			logger.Error(err, "Node not found. Wrong name provided or this node did not yet join the cluster, continuing anyways", "nodeName", o.NodeName)
			nodeHostname = o.NodeName
		} else {
			return fmt.Errorf("failed to determine hostname for node: %w", err)
		}
	}

	// prepare Bastion resource
	policies, err := o.bastionIngressPolicies(logger, shoot.Spec.Provider.Type)
	if err != nil {
		return fmt.Errorf("failed to get bastion ingress policies: %w", err)
	}

	sshPublicKey, err := os.ReadFile(o.SSHPublicKeyFile.String())
	if err != nil {
		return fmt.Errorf("failed to read SSH public key: %w", err)
	}

	bastionKey := client.ObjectKey{
		Namespace: shoot.Namespace,
		Name:      o.BastionName,
	}

	// allow to cancel at any time, but with us still performing the cleanup
	signalChan := createSignalChannel()

	go func() {
		<-signalChan

		// If this goroutine caught the signal, the waitForSignal() might get
		// stuck waiting for _another_ signal. To prevent this deadlock, we
		// simply close the channel and "trigger" all who wait for it.
		close(signalChan)

		logger.Info("Caught signal, cancelling...")
		cancel()
	}()

	// do not use `ctx`, as it might be cancelled already when running the cleanup
	defer cleanup(f.Context(), o, gardenClient.RuntimeClient(), bastionKey, nodePrivateKeyFiles)

	bastion, err := createOrPatchBastion(ctx, gardenClient.RuntimeClient(), bastionKey, shoot, sshPublicKey, policies)
	if err != nil {
		return err
	}

	// continuously keep the bastion alive by renewing its annotation
	go keepBastionAlive(ctx, cancel, gardenClient.RuntimeClient(), bastion.DeepCopy())

	logger.Info("Waiting for bastion to be ready…", "waitTimeout", o.WaitTimeout)

	err = waitForBastion(ctx, o, gardenClient.RuntimeClient(), bastion)
	if wait.Interrupted(err) {
		return errors.New("timed out waiting for the bastion to be ready")
	} else if err != nil {
		return fmt.Errorf("an error occurred while waiting for the bastion to be ready: %w", err)
	}

	logger.Info("Bastion host became available.", "address", toAddress(bastion.Status.Ingress).String())

	bastionPreferredAddress := preferredBastionAddress(o.BastionHost, bastion)

	if !o.Interactive {
		var nodes []corev1.Node

		var pendingNodeNames []string

		if nodeHostname == "" {
			nodes, err = getNodes(ctx, shootClient)
			if err != nil {
				return fmt.Errorf("failed to list shoot cluster nodes: %w", err)
			}

			// We also want to determine if there are nodes that have not yet joined the cluster.
			// This is an optional step, as only Gardener operators have access to the Seeds.
			// Regular users will receive a 'Forbidden' error when trying to fetch the Machines.
			// However, we do not want to log an error message in this case.
			pendingNodeNames, err = getNodeNamesFromMachines(ctx, manager, currentTarget)
			if err != nil && !apierrors.IsForbidden(err) {
				logger.Info("failed to get shoot cluster node names from machines", "err", err)
			}
		}

		connectInformation, err := NewConnectInformation(
			bastion,
			bastionPreferredAddress,
			o.BastionPort,
			o.BastionUserKnownHostsFiles,
			nodeHostname,
			o.SSHPublicKeyFile,
			o.SSHPrivateKeyFile,
			nodePrivateKeyFiles,
			nodes,
			pendingNodeNames,
			o.User,
		)
		if err != nil {
			return err
		}

		if err := o.PrintObject(connectInformation); err != nil {
			return err
		}

		if o.NoKeepalive {
			return nil
		}

		waitForSignal(ctx, o, ctx.Done())

		return nil
	}

	return remoteShell(
		ctx,
		o.IOStreams,
		bastionPreferredAddress,
		o.BastionPort,
		o.SSHPrivateKeyFile,
		o.BastionUserKnownHostsFiles,
		nodeHostname,
		nodePrivateKeyFiles,
		o.User,
	)
}

func createOrPatchBastion(ctx context.Context, gardenClient client.Client, key client.ObjectKey, shoot *gardencorev1beta1.Shoot, sshPublicKey []byte, policies []operationsv1alpha1.BastionIngressPolicy) (*operationsv1alpha1.Bastion, error) {
	logger := klog.FromContext(ctx)

	bastion := &operationsv1alpha1.Bastion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
	}

	result, err := controllerutil.CreateOrPatch(ctx, gardenClient, bastion, func() error {
		if len(bastion.Annotations) == 0 {
			bastion.Annotations = map[string]string{}
		}

		bastion.Annotations[corev1beta1constants.GardenerOperation] = corev1beta1constants.GardenerOperationKeepalive
		bastion.Spec.ShootRef = corev1.LocalObjectReference{
			Name: shoot.Name,
		}
		bastion.Spec.SSHPublicKey = strings.TrimSpace(string(sshPublicKey))
		bastion.Spec.Ingress = policies

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create or patch bastion: %w", err)
	}

	logger.Info("Bastion operation", "bastion", klog.KObj(bastion), "result", result)

	return bastion, nil
}

func (o *SSHOptions) bastionIngressPolicies(logger klog.Logger, providerType string) ([]operationsv1alpha1.BastionIngressPolicy, error) {
	var policies []operationsv1alpha1.BastionIngressPolicy

	for _, cidr := range o.CIDRs {
		if providerType == "gcp" {
			ip, _, err := net.ParseCIDR(cidr)
			if err != nil {
				return nil, err // this should never happen, as it is already checked within the Validate function
			}

			if ip.To4() == nil {
				if !o.AutoDetected {
					return nil, fmt.Errorf("GCP only supports IPv4: %s", cidr)
				}

				logger.Info("GCP only supports IPv4, skipped CIDR: %s\n", "cidr", cidr)

				continue // skip
			}
		}

		policies = append(policies, operationsv1alpha1.BastionIngressPolicy{
			IPBlock: networkingv1.IPBlock{
				CIDR: cidr,
			},
		})
	}

	if len(policies) == 0 {
		return nil, errors.New("no ingress policies left")
	}

	return policies, nil
}

func printTargetInformation(logger klog.Logger, t target.Target) {
	var step string

	if t.ProjectName() != "" {
		step = t.ProjectName()
	} else {
		step = t.SeedName()
	}

	target := fmt.Sprintf("%s/%s", step, t.ShootName()) // klog.KRef() does not really fit as it expects a namespace
	logger.Info("Preparing SSH access", "target", target, "garden", t.GardenName())
}

func cleanup(ctx context.Context, o *SSHOptions, gardenClient client.Client, bastionKey client.ObjectKey, nodePrivateKeyFiles []PrivateKeyFile) {
	logger := klog.FromContext(ctx)

	if !o.KeepBastion {
		logger.Info("Cleaning up")

		bastion := &operationsv1alpha1.Bastion{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bastionKey.Name,
				Namespace: bastionKey.Namespace,
			},
		}
		if err := gardenClient.Delete(ctx, bastion); client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to delete bastion.", "bastion", klog.KObj(bastion))
		}

		if o.GeneratedSSHKeys {
			if err := os.Remove(o.SSHPublicKeyFile.String()); err != nil {
				logger.Error(err, "Failed to delete SSH public key file", "path", o.SSHPublicKeyFile)
			}

			if err := os.Remove(o.SSHPrivateKeyFile.String()); err != nil {
				logger.Error(err, "Failed to delete SSH private key file", "path", o.SSHPrivateKeyFile)
			}
		}

		// though technically not used _on_ the bastion itself, without
		// these files remaining, the user would not be able to use the SSH
		// command we provided to connect to the shoot nodes
		for _, filename := range nodePrivateKeyFiles {
			if err := os.Remove(filename.String()); err != nil {
				logger.Error(err, "Failed to delete node private key", "path", filename)
			}
		}
	} else {
		logger.Info("Keeping bastion", "bastion", klog.KRef(bastionKey.Namespace, bastionKey.Name))

		if o.GeneratedSSHKeys {
			logger.Info("The SSH keypair for the bastion remain on disk", "publicKeyPath", o.SSHPublicKeyFile, "privateKeyPath", o.SSHPrivateKeyFile)
		}

		logger.Info("The private SSH keys for shoot nodes remain on disk", "paths", nodePrivateKeyFiles)
	}
}

func getNodeNamesFromMachinesOrNodes(ctx context.Context, manager target.Manager) ([]string, error) {
	logger := klog.FromContext(ctx)

	currentTarget, err := manager.CurrentTarget()
	if err != nil {
		return nil, err
	}

	if currentTarget.ShootName() == "" {
		return nil, errors.New("no Shoot cluster targeted")
	}

	nodeNames, err := getNodeNamesFromMachines(ctx, manager, currentTarget)
	if err != nil {
		// Regular users do not have the permission to fetch the machines.
		// However, in this case, we do not want to log an error message. Instead, we will fallback to read the node names.
		if !apierrors.IsForbidden(err) {
			logger.Info("failed to fetch node names from machine objects", "err", err)
		}

		return getNodeNamesFromNodes(ctx, manager, currentTarget)
	}

	return nodeNames, nil
}

func getNodeNamesFromMachines(ctx context.Context, manager target.Manager, currentTarget target.Target) ([]string, error) {
	gardenClient, err := manager.GardenClient(currentTarget.GardenName())
	if err != nil {
		return nil, fmt.Errorf("failed to create garden cluster client: %w", err)
	}

	shoot, err := gardenClient.FindShoot(ctx, currentTarget.AsListOption())
	if err != nil {
		return nil, err
	}

	seedTarget := target.NewTarget(currentTarget.GardenName(), "", *shoot.Spec.SeedName, "")

	seedClient, err := manager.SeedClient(ctx, seedTarget)
	if err != nil {
		return nil, err
	}

	machines, err := getMachines(ctx, seedClient, shoot.Status.TechnicalID)
	if err != nil {
		return nil, err
	}

	var nodeNames []string

	for _, machine := range machines {
		if _, ok := machine.Labels[machinev1alpha1.NodeLabelKey]; !ok {
			continue
		}

		nodeNames = append(nodeNames, machine.Labels[machinev1alpha1.NodeLabelKey])
	}

	return nodeNames, nil
}

func getNodeNamesFromNodes(ctx context.Context, manager target.Manager, currentTarget target.Target) ([]string, error) {
	shootClient, err := manager.ShootClient(ctx, currentTarget)
	if err != nil {
		return nil, err
	}

	nodes, err := getNodes(ctx, shootClient)
	if err != nil {
		return nil, err
	}

	var nodeNames []string
	for _, node := range nodes {
		nodeNames = append(nodeNames, node.Name)
	}

	return nodeNames, nil
}

// preferredBastionAddress returns the preferred bastion address.
//
// Selection Priority:
// 1. bastionHostOverride (if non-empty)
// 2. bastion IP (if available)
// 3. bastion hostname (if available)
// 4. empty string (if none of the above options are available).
func preferredBastionAddress(bastionHostOverride string, bastion *operationsv1alpha1.Bastion) string {
	if bastionHostOverride != "" {
		return bastionHostOverride
	}

	if ingress := bastion.Status.Ingress; ingress != nil {
		if ingress.IP != "" {
			return ingress.IP
		}

		return ingress.Hostname
	}

	return ""
}

func waitForBastion(ctx context.Context, o *SSHOptions, gardenClient client.Client, bastion *operationsv1alpha1.Bastion) error {
	var (
		lastCheckErr    error
		privateKeyBytes []byte
		err             error
	)

	logger := klog.FromContext(ctx)

	if o.SSHPrivateKeyFile != "" {
		privateKeyBytes, err = os.ReadFile(o.SSHPrivateKeyFile.String())
		if err != nil {
			return fmt.Errorf("failed to read SSH private key from %q: %w", o.SSHPrivateKeyFile, err)
		}
	}

	waitErr := wait.PollUntilContextTimeout(ctx, pollBastionStatusInterval, o.WaitTimeout, false, func(ctx context.Context) (bool, error) {
		key := client.ObjectKeyFromObject(bastion)

		if err := gardenClient.Get(ctx, key, bastion); err != nil {
			return false, err
		}

		switch cond := corev1beta1helper.GetCondition(bastion.Status.Conditions, operationsv1alpha1.BastionReady); {
		case cond == nil:
			return false, nil
		case cond.Status != gardencorev1beta1.ConditionTrue:
			lastCheckErr = errors.New(cond.Message)
			logger.Error(lastCheckErr, "Still waiting")

			return false, nil
		}

		if o.SkipAvailabilityCheck {
			logger.Info("Bastion is ready, skipping availability check")
			return true, nil
		}

		bastionPreferredAddress := preferredBastionAddress(o.BastionHost, bastion)

		lastCheckErr = bastionAvailabilityChecker(bastionPreferredAddress, o.BastionPort, privateKeyBytes)
		if lastCheckErr != nil {
			logger.Error(lastCheckErr, "Still waiting for bastion to accept SSH connection")
			return false, nil
		}

		return true, nil
	})

	if wait.Interrupted(waitErr) {
		return fmt.Errorf("timed out waiting for the bastion to become ready: %w", lastCheckErr)
	}

	return waitErr
}

func getShootNode(ctx context.Context, o *SSHOptions, shootClient client.Client) (*corev1.Node, error) {
	node := &corev1.Node{}
	if err := shootClient.Get(ctx, types.NamespacedName{Name: o.NodeName}, node); err != nil {
		return nil, err
	}

	return node, nil
}

func remoteShell(
	ctx context.Context,
	ioStreams util.IOStreams,
	bastionHost string,
	bastionPort string,
	sshPrivateKeyFile PrivateKeyFile,
	bastionUserKnownHostsFiles []string,
	nodeHostname string,
	nodePrivateKeyFiles []PrivateKeyFile,
	user string,
) error {
	commandArgs := sshCommandArguments(
		bastionHost,
		bastionPort,
		sshPrivateKeyFile,
		bastionUserKnownHostsFiles,
		nodeHostname,
		nodePrivateKeyFiles,
		user,
	)

	fmt.Fprintf(ioStreams.Out, "> You can open additional SSH sessions by running the following command in a separate terminal:\n\n")
	fmt.Fprintf(ioStreams.Out, "ssh %s\n\n", commandArgs.String())

	var args []string

	for _, arg := range commandArgs.list {
		args = append(args, arg.value)
	}

	return execCommand(ctx, "ssh", args, ioStreams)
}

func getKeepAliveInterval() time.Duration {
	keepAliveIntervalMutex.RLock()
	defer keepAliveIntervalMutex.RUnlock()

	return keepAliveInterval
}

func keepBastionAlive(ctx context.Context, cancel context.CancelFunc, gardenClient client.Client, bastion *operationsv1alpha1.Bastion) {
	logger := klog.FromContext(ctx).WithValues("bastion", klog.KObj(bastion))

	ticker := time.NewTicker(getKeepAliveInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			// re-fetch current bastion
			key := types.NamespacedName{Name: bastion.Name, Namespace: bastion.Namespace}

			// reset annotations so that we fetch the actual current state
			bastion.Annotations = map[string]string{}

			if err := gardenClient.Get(ctx, key, bastion); err != nil {
				if apierrors.IsNotFound(err) {
					logger.Error(err, "Can't keep bastion alive. Bastion is already gone.")
					cancel()

					return
				}

				logger.Error(err, "Failed to keep bastion alive.")
			}

			// add the keepalive annotation
			oldBastion := bastion.DeepCopy()

			if bastion.Annotations == nil {
				bastion.Annotations = map[string]string{}
			}

			bastion.Annotations[corev1beta1constants.GardenerOperation] = corev1beta1constants.GardenerOperationKeepalive

			if err := gardenClient.Patch(ctx, bastion, client.MergeFrom(oldBastion)); err != nil {
				logger.Error(err, "Failed to keep bastion alive.")
			}
		}
	}
}

func getShootNodePrivateKeys(ctx context.Context, gardenClient client.Client, shoot *gardencorev1beta1.Shoot) ([][]byte, error) {
	keys := [][]byte{}

	// TODO: use ShootProjectSecretSuffixOldSSHKeypair once Gardener releases a version with it
	for _, suffix := range []string{gutil.ShootProjectSecretSuffixSSHKeypair, "ssh-keypair.old"} {
		secret := &corev1.Secret{}
		key := types.NamespacedName{
			Name:      fmt.Sprintf("%s.%s", shoot.Name, suffix),
			Namespace: shoot.Namespace,
		}

		if err := gardenClient.Get(ctx, key, secret); client.IgnoreNotFound(err) != nil {
			return nil, err
		}

		if secret.Name != "" {
			keys = append(keys, secret.Data[secrets.DataKeyRSAPrivateKey])
		}
	}

	if len(keys) == 0 {
		return nil, errors.New("no SSH keypair is available for the shoot nodes")
	}

	return keys, nil
}

func writeToTemporaryFile(key []byte) (string, error) {
	f, err := tempFileCreator()
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := f.Write(key); err != nil {
		return "", err
	}

	if err := f.Sync(); err != nil {
		return "", err
	}

	return f.Name(), nil
}

func getNodeHostname(node *corev1.Node) (string, error) {
	addresses := map[corev1.NodeAddressType]string{}
	for _, addr := range node.Status.Addresses {
		addresses[addr.Type] = addr.Address
	}

	// As we connect via a jump host that's in the same network
	// as the shoot nodes, we prefer the internal IP/hostname.
	for _, k := range []corev1.NodeAddressType{corev1.NodeInternalIP, corev1.NodeInternalDNS, corev1.NodeExternalIP, corev1.NodeExternalDNS} {
		if addr := addresses[k]; addr != "" {
			return addr, nil
		}
	}

	return "", errors.New("node has no internal or external names")
}

func getNodes(ctx context.Context, c client.Client) ([]corev1.Node, error) {
	nodeList := corev1.NodeList{}
	if err := c.List(ctx, &nodeList, &client.ListOptions{}); err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	return nodeList.Items, nil
}

func getMachines(ctx context.Context, c client.Client, namespace string) ([]machinev1alpha1.Machine, error) {
	machineList := machinev1alpha1.MachineList{}
	if err := c.List(ctx, &machineList, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("failed to list machines: %w", err)
	}

	return machineList.Items, nil
}

func (o *SSHOptions) checkAccessRestrictions(cfg *config.Config, gardenName string, tf target.TargetFlags, shoot *gardencorev1beta1.Shoot) (bool, error) {
	if cfg == nil {
		return false, errors.New("garden configuration is required")
	}

	if tf == nil {
		return false, errors.New("target flags are required")
	}

	// handle access restrictions
	garden, err := cfg.Garden(gardenName)
	if err != nil {
		return false, err
	}

	askForConfirmation := tf.ShootName() != "" && !o.ConfirmAccessRestriction
	handler := ac.NewAccessRestrictionHandler(o.IOStreams.In, o.IOStreams.ErrOut, askForConfirmation) // do not write access restriction to stdout, otherwise it would break the output format

	return handler(ac.CheckAccessRestrictions(garden.AccessRestrictions, shoot)), nil
}
