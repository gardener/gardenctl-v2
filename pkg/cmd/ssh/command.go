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
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/target"

	"github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	corev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/printers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// SSHBastionUsername is the system username on the bastion host.
	SSHBastionUsername = "gardener"
	// SSHNodeUsername is the system username on any of the shoot cluster nodes.
	SSHNodeUsername = "gardener"
	// SSHPort is the TCP port on a bastion instance that allows incoming SSH.
	SSHPort = 22
)

// wrappers used for unit tests only
var (
	// keepAliveInterval is the interval in which bastions should be given the
	// keep-alive annotation to prolong their lifetime.
	keepAliveInterval = 3 * time.Minute

	// pollBastionStatusInterval is the time in-between status checks on the bastion object.
	pollBastionStatusInterval = 5 * time.Second

	// tempFileCreator creates and opens a temporary file
	tempFileCreator = func() (*os.File, error) {
		return os.CreateTemp(os.TempDir(), "gctlv2*")
	}

	// bastionAvailabilityChecker returns nil if the given hostname allows incoming
	// connections on the SSHPort and has a public key configured that matches the
	// given private key.
	bastionAvailabilityChecker = func(hostname string, privateKey []byte) error {
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

		client, err := ssh.Dial("tcp", net.JoinHostPort(hostname, strconv.Itoa(SSHPort)), &ssh.ClientConfig{
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
	// from the options. The function returns an error if the command
	// fails.
	execCommand = func(ctx context.Context, command string, args []string, o *Options) error {
		cmd := exec.CommandContext(ctx, command, args...)
		cmd.Stdout = o.IOStreams.Out
		cmd.Stdin = o.IOStreams.In
		cmd.Stderr = o.IOStreams.ErrOut

		return cmd.Run()
	}
)

// NewCommand returns a new ssh command.
func NewCommand(f util.Factory, o *Options) *cobra.Command {
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
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(f, cmd, args, o.IOStreams.Out); err != nil {
				return fmt.Errorf("failed to complete command options: %w", err)
			}
			if err := o.Validate(); err != nil {
				return err
			}

			return runCommand(f, o)
		},
	}

	cmd.Flags().BoolVar(&o.Interactive, "interactive", o.Interactive, "Open an SSH connection instead of just providing the bastion host (only if NODE_NAME is provided).")
	cmd.Flags().StringArrayVar(&o.CIDRs, "cidr", nil, "CIDRs to allow access to the bastion host; if not given, your system's public IP is auto-detected.")
	cmd.Flags().StringVar(&o.SSHPublicKeyFile, "public-key-file", "", "Path to the file that contains a public SSH key. If not given, a temporary keypair will be generated.")
	cmd.Flags().DurationVar(&o.WaitTimeout, "wait-timeout", o.WaitTimeout, "Maximum duration to wait for the bastion to become available.")
	cmd.Flags().BoolVar(&o.KeepBastion, "keep-bastion", o.KeepBastion, "Do not delete immediately when gardenctl exits (Bastions will be garbage-collected after some time)")

	return cmd
}

func runCommand(f util.Factory, o *Options) error {
	manager, err := f.Manager()
	if err != nil {
		return err
	}

	// validate the current target
	currentTarget, err := manager.CurrentTarget()
	if err != nil {
		return err
	}

	if currentTarget.ShootName() == "" {
		return errors.New("no Shoot cluster targeted")
	}

	printTargetInformation(o.IOStreams.Out, currentTarget)

	// create client for the garden cluster
	gardenClient, err := manager.GardenClient(currentTarget)
	if err != nil {
		return err
	}

	// fetch targeted shoot (ctx is cancellable to stop the keep alive goroutine later)
	ctx, cancel := context.WithCancel(f.Context())
	defer cancel()

	shoot, err := util.ShootForTarget(ctx, gardenClient, currentTarget)
	if err != nil {
		return err
	}

	// fetch the SSH key(s) for the shoot nodes
	nodePrivateKeys, err := getShootNodePrivateKeys(ctx, gardenClient, shoot)
	if err != nil {
		return err
	}

	// save the keys into temporary files that we try to clean up when exiting
	nodePrivateKeyFiles := []string{}

	for _, pk := range nodePrivateKeys {
		filename, err := writeToTemporaryFile(pk)
		if err != nil {
			return err
		}

		nodePrivateKeyFiles = append(nodePrivateKeyFiles, filename)
	}

	shootClient, err := manager.ShootClusterClient(ctx, currentTarget)
	if err != nil {
		return err
	}

	// if a node was given, check if the node exists
	// and if not, exit early and do not create a bastion
	node, err := getShootNode(ctx, o, shootClient, currentTarget)
	if err != nil {
		return err
	}

	// prepare Bastion resource
	policies := []operationsv1alpha1.BastionIngressPolicy{}

	for _, cidr := range o.CIDRs {
		policies = append(policies, operationsv1alpha1.BastionIngressPolicy{
			IPBlock: networkingv1.IPBlock{
				CIDR: cidr,
			},
		})
	}

	sshPublicKey, err := ioutil.ReadFile(o.SSHPublicKeyFile)
	if err != nil {
		return fmt.Errorf("failed to read SSH public key: %v", err)
	}

	// avoid GenerateName because we want to immediately fetch and check the bastion
	bastionName, err := bastionNameProvider()
	if err != nil {
		return fmt.Errorf("failed to create bastion name: %v", err)
	}

	bastion := &operationsv1alpha1.Bastion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bastionName,
			Namespace: shoot.Namespace,
		},
		Spec: operationsv1alpha1.BastionSpec{
			ShootRef: corev1.LocalObjectReference{
				Name: shoot.Name,
			},
			SSHPublicKey: strings.TrimSpace(string(sshPublicKey)),
			Ingress:      policies,
		},
	}

	// allow to cancel at any time, but with us still performing the cleanup
	signalChan := createSignalChannel()

	go func() {
		<-signalChan

		// If this goroutine caught the signal, the waitForSignal() might get
		// stuck waiting for _another_ signal. To prevent this deadlock, we
		// simply close the channel and "trigger" all who wait for it.
		close(signalChan)

		fmt.Fprintln(o.IOStreams.Out, "Caught signal, cancelling...")
		cancel()
	}()

	// do not use `ctx`, as it might be cancelled already when running the cleanup
	defer cleanup(f.Context(), o, gardenClient, bastion, nodePrivateKeyFiles)

	fmt.Fprintf(o.IOStreams.Out, "Creating bastion %s…\n", bastion.Name)

	if err := gardenClient.Create(ctx, bastion); err != nil {
		return fmt.Errorf("failed to create bastion: %v", err)
	}

	// continuously keep the bastion alive by renewing its annotation
	go keepBastionAlive(ctx, gardenClient, bastion.DeepCopy(), o.IOStreams.ErrOut)

	fmt.Fprintf(o.IOStreams.Out, "Waiting up to %v for bastion to be ready…\n", o.WaitTimeout)

	err = waitForBastion(ctx, o, gardenClient, bastion)

	if err == wait.ErrWaitTimeout {
		fmt.Fprintln(o.IOStreams.Out, "Timed out waiting for the bastion to be ready.")
	} else if err != nil {
		fmt.Fprintf(o.IOStreams.Out, "An error occurred while waiting: %v\n", err)
	}

	if err != nil {
		// actual error has already been printed
		return errors.New("precondition failed")
	}

	ingress := bastion.Status.Ingress
	printAddr := ""

	if ingress.Hostname != "" && ingress.IP != "" {
		printAddr = fmt.Sprintf("%s (%s)", ingress.IP, ingress.Hostname)
	} else if ingress.Hostname != "" {
		printAddr = ingress.Hostname
	} else {
		printAddr = ingress.IP
	}

	fmt.Fprintf(o.IOStreams.Out, "Bastion host became available at %s.\n", printAddr)

	if node != nil && o.Interactive {
		err = remoteShell(ctx, o, bastion, nodePrivateKeyFiles, node)
	} else {
		err = waitForSignal(ctx, o, shootClient, bastion, nodePrivateKeyFiles, node, ctx.Done())
	}

	fmt.Fprintln(o.IOStreams.Out, "Exiting…")

	return err
}

func printTargetInformation(out io.Writer, t target.Target) {
	var step string

	if t.ProjectName() != "" {
		step = t.ProjectName()
	} else {
		step = t.SeedName()
	}

	fmt.Fprintf(out, "Preparing SSH access to %s/%s on %s…\n", step, t.ShootName(), t.GardenName())
}

func cleanup(ctx context.Context, o *Options, gardenClient client.Client, bastion *operationsv1alpha1.Bastion, nodePrivateKeyFiles []string) {
	if !o.KeepBastion {
		fmt.Fprintf(o.IOStreams.Out, "Deleting bastion %s…\n", bastion.Name)

		if err := gardenClient.Delete(ctx, bastion); client.IgnoreNotFound(err) != nil {
			fmt.Fprintf(o.IOStreams.ErrOut, "Failed to delete bastion: %v", err)
		}

		if o.generatedSSHKeys {
			if err := os.Remove(o.SSHPublicKeyFile); err != nil {
				fmt.Fprintf(o.IOStreams.ErrOut, "Failed to delete SSH public key file %q: %v\n", o.SSHPublicKeyFile, err)
			}

			if err := os.Remove(o.SSHPrivateKeyFile); err != nil {
				fmt.Fprintf(o.IOStreams.ErrOut, "Failed to delete SSH private key file %q: %v\n", o.SSHPrivateKeyFile, err)
			}
		}

		// though technically not used _on_ the bastion itself, without
		// these files remaining, the user would not be able to use the SSH
		// command we provided to connect to the shoot nodes
		for _, filename := range nodePrivateKeyFiles {
			if err := os.Remove(filename); err != nil {
				fmt.Fprintf(o.IOStreams.ErrOut, "Failed to delete node private key %q: %v\n", filename, err)
			}
		}
	} else {
		fmt.Fprintf(o.IOStreams.Out, "Keeping bastion %s in namespace %s.\n", bastion.Name, bastion.Namespace)

		if o.generatedSSHKeys {
			fmt.Fprintf(o.IOStreams.Out, "The SSH keypair for the bastion is stored at %s (public key) and %s (private key).\n", o.SSHPublicKeyFile, o.SSHPrivateKeyFile)
		}

		fmt.Fprintf(o.IOStreams.Out, "The private SSH keys for shoot nodes are stored at %s.\n", strings.Join(nodePrivateKeyFiles, ", "))
	}
}

func getNodeNamesFromShoot(f util.Factory, prefix string) ([]string, error) {
	manager, err := f.Manager()
	if err != nil {
		return nil, err
	}

	// validate the current target
	currentTarget, err := manager.CurrentTarget()
	if err != nil {
		return nil, err
	}

	if currentTarget.ShootName() == "" {
		return nil, errors.New("no Shoot cluster targeted")
	}

	// create client for the shoot cluster
	shootClient, err := manager.ShootClusterClient(f.Context(), currentTarget)
	if err != nil {
		return nil, err
	}

	// fetch all nodes
	nodes, err := getNodes(f.Context(), shootClient)
	if err != nil {
		return nil, err
	}

	// collect names, filter by prefix
	nodeNames := []string{}

	for _, node := range nodes {
		if strings.HasPrefix(node.Name, prefix) {
			nodeNames = append(nodeNames, node.Name)
		}
	}

	return nodeNames, nil
}

func preferredBastionAddress(bastion *operationsv1alpha1.Bastion) string {
	if ingress := bastion.Status.Ingress; ingress != nil {
		if ingress.IP != "" {
			return ingress.IP
		}

		return ingress.Hostname
	}

	return ""
}

func waitForBastion(ctx context.Context, o *Options, gardenClient client.Client, bastion *operationsv1alpha1.Bastion) error {
	var (
		lastCheckErr    error
		privateKeyBytes []byte
		err             error
	)

	if o.SSHPrivateKeyFile != "" {
		privateKeyBytes, err = ioutil.ReadFile(o.SSHPrivateKeyFile)
		if err != nil {
			return fmt.Errorf("failed to read SSH private key from %q: %w", o.SSHPrivateKeyFile, err)
		}
	}

	waitErr := wait.Poll(pollBastionStatusInterval, o.WaitTimeout, func() (bool, error) {
		key := client.ObjectKeyFromObject(bastion)

		if err := gardenClient.Get(ctx, key, bastion); err != nil {
			return false, err
		}

		cond := corev1alpha1helper.GetCondition(bastion.Status.Conditions, operationsv1alpha1.BastionReady)

		if cond == nil || cond.Status != v1alpha1.ConditionTrue {
			lastCheckErr = errors.New("bastion does not have BastionReady=true condition")
			fmt.Fprintf(o.IOStreams.ErrOut, "Still waiting: %v\n", lastCheckErr)
			return false, nil
		}

		lastCheckErr = bastionAvailabilityChecker(preferredBastionAddress(bastion), privateKeyBytes)
		if lastCheckErr != nil {
			fmt.Fprintf(o.IOStreams.ErrOut, "Still waiting: cannot connect to bastion yet: %v\n", lastCheckErr)
			return false, nil
		}

		return true, nil
	})

	if waitErr == wait.ErrWaitTimeout {
		return fmt.Errorf("timed out waiting for the bastion to become ready: %w", lastCheckErr)
	}

	return waitErr
}

func getShootNode(ctx context.Context, o *Options, shootClient client.Client, currentTarget target.Target) (*corev1.Node, error) {
	if o.NodeName == "" {
		return nil, nil
	}

	node := &corev1.Node{}
	if err := shootClient.Get(ctx, types.NamespacedName{Name: o.NodeName}, node); err != nil {
		return nil, err
	}

	return node, nil
}

func remoteShell(ctx context.Context, o *Options, bastion *operationsv1alpha1.Bastion, nodePrivateKeyFiles []string, node *corev1.Node) error {
	nodeHostname, err := getNodeHostname(node)
	if err != nil {
		return err
	}

	bastionAddr := preferredBastionAddress(bastion)
	connectCmd := sshCommandLine(o, bastionAddr, nodePrivateKeyFiles, nodeHostname)

	fmt.Fprintln(o.IOStreams.Out, "You can open additional SSH sessions using the command below:")
	fmt.Fprintln(o.IOStreams.Out, "")
	fmt.Fprintln(o.IOStreams.Out, connectCmd)
	fmt.Fprintln(o.IOStreams.Out, "")

	proxyPrivateKeyFlag := ""
	if o.SSHPrivateKeyFile != "" {
		proxyPrivateKeyFlag = fmt.Sprintf(" -o IdentitiesOnly=yes -i %s", o.SSHPrivateKeyFile)
	}

	proxyCmd := fmt.Sprintf(
		"ssh -W%%h:%%p -o StrictHostKeyChecking=no%s %s@%s",
		proxyPrivateKeyFlag,
		SSHBastionUsername,
		bastionAddr,
	)

	args := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "IdentitiesOnly=yes",
		"-o", fmt.Sprintf("ProxyCommand=%s", proxyCmd),
	}

	for _, file := range nodePrivateKeyFiles {
		args = append(args, "-i", file)
	}

	args = append(args, fmt.Sprintf("%s@%s", SSHNodeUsername, nodeHostname))

	return execCommand(ctx, "ssh", args, o)
}

// waitForSignal informs the user about their options and keeps the
// bastion alive until gardenctl exits.
func waitForSignal(ctx context.Context, o *Options, shootClient client.Client, bastion *operationsv1alpha1.Bastion, nodePrivateKeyFiles []string, node *corev1.Node, signalChan <-chan struct{}) error {
	nodeHostname := "IP_OR_HOSTNAME"

	if node != nil {
		var err error

		nodeHostname, err = getNodeHostname(node)
		if err != nil {
			return fmt.Errorf("failed to determine hostname for node: %w", err)
		}
	}

	bastionAddr := preferredBastionAddress(bastion)
	connectCmd := sshCommandLine(o, bastionAddr, nodePrivateKeyFiles, nodeHostname)

	if node == nil {
		nodes, err := getNodes(ctx, shootClient)
		if err != nil {
			return fmt.Errorf("failed to list shoot cluster nodes: %w", err)
		}

		table := &metav1beta1.Table{
			ColumnDefinitions: []metav1.TableColumnDefinition{
				{
					Name:   "Node Name",
					Type:   "string",
					Format: "name",
				},
				{
					Name: "Status",
					Type: "string",
				},
				{
					Name: "IP",
					Type: "string",
				},
				{
					Name: "Hostname",
					Type: "string",
				},
			},
			Rows: []metav1.TableRow{},
		}

		for _, node := range nodes {
			ip := ""
			hostname := ""
			status := "Ready"

			if !isNodeReady(node) {
				status = "Not Ready"
			}

			for _, addr := range node.Status.Addresses {
				switch addr.Type {
				case corev1.NodeInternalIP:
					ip = addr.Address

				case corev1.NodeInternalDNS:
					hostname = addr.Address

				// internal names have priority, as we jump via a bastion host,
				// but in case the cloud provider does not offer internal IPs,
				// we fallback to external values

				case corev1.NodeExternalIP:
					if ip == "" {
						ip = addr.Address
					}

				case corev1.NodeExternalDNS:
					if hostname == "" {
						hostname = addr.Address
					}
				}
			}

			table.Rows = append(table.Rows, metav1.TableRow{
				Cells: []interface{}{node.Name, status, ip, hostname},
			})
		}

		fmt.Fprintln(o.IOStreams.Out, "The shoot cluster has the following nodes:")
		fmt.Fprintln(o.IOStreams.Out, "")

		printer := printers.NewTablePrinter(printers.PrintOptions{})
		if err := printer.PrintObj(table, o.IOStreams.Out); err != nil {
			return fmt.Errorf("failed to output node table: %w", err)
		}

		fmt.Fprintln(o.IOStreams.Out, "")
	}

	fmt.Fprintln(o.IOStreams.Out, "Connect to shoot nodes by using the bastion as a proxy/jump host, for example:")
	fmt.Fprintln(o.IOStreams.Out, "")
	fmt.Fprintln(o.IOStreams.Out, connectCmd)
	fmt.Fprintln(o.IOStreams.Out, "")

	fmt.Fprintln(o.IOStreams.Out, "Press Ctrl-C to stop gardenctl, after which the bastion will be removed.")

	<-signalChan

	return nil
}

func isNodeReady(node corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			return cond.Status == corev1.ConditionTrue
		}
	}

	return false
}

func sshCommandLine(o *Options, bastionAddr string, nodePrivateKeyFiles []string, nodeName string) string {
	proxyPrivateKeyFlag := ""
	if o.SSHPrivateKeyFile != "" {
		proxyPrivateKeyFlag = fmt.Sprintf(" -o IdentitiesOnly=yes -i %s", o.SSHPrivateKeyFile)
	}

	proxyCmd := fmt.Sprintf(
		"ssh -W%%h:%%p -o StrictHostKeyChecking=no%s %s@%s",
		proxyPrivateKeyFlag,
		SSHBastionUsername,
		bastionAddr,
	)

	identities := []string{}
	for _, filename := range nodePrivateKeyFiles {
		identities = append(identities, fmt.Sprintf("-i %s", filename))
	}

	connectCmd := fmt.Sprintf(
		`ssh -o "StrictHostKeyChecking=no" -o "IdentitiesOnly=yes" %s -o "ProxyCommand=%s" %s@%s`,
		strings.Join(identities, " "),
		proxyCmd,
		SSHNodeUsername,
		nodeName,
	)

	return connectCmd
}

func keepBastionAlive(ctx context.Context, gardenClient client.Client, bastion *operationsv1alpha1.Bastion, stderr io.Writer) {
	ticker := time.NewTicker(keepAliveInterval)
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
				fmt.Fprintf(stderr, "Failed to keep bastion alive: %v\n", err)
			}

			// add the keepalive annotation
			oldBastion := bastion.DeepCopy()
			bastion.Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.GardenerOperationKeepalive

			if err := gardenClient.Patch(ctx, bastion, client.MergeFrom(oldBastion)); err != nil {
				fmt.Fprintf(stderr, "Failed to keep bastion alive: %v\n", err)
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
