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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	corev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	// SSHBastionUsername is the system username on the bastion host.
	SSHBastionUsername = "gardener"
	// SSHNodeUsername is the system username on any of the shoot cluster nodes.
	SSHNodeUsername = "gardener"
	// SSHPort is the TCP port on a bastion instance that allows incoming SSH.
	SSHPort = 22
)

// NewCommand returns a new ssh command.
func NewCommand(f util.Factory, o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "Establish an SSH connection to a Shoot cluster's node",
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

	cmd.Flags().BoolVar(&o.Interactive, "interactive", o.Interactive, "Open an SSH connection instead of just providing the bastion host.")
	cmd.Flags().StringArrayVar(&o.CIDRs, "cidr", nil, "CIDRs to allow access to the bastion host; if not given, the host's public IP is auto-detected.")
	cmd.Flags().StringVar(&o.SSHPublicKeyFile, "public-key-file", "", "Path to the file that contains a public SSH key. If not given, a temporary keypair will be generated.")
	cmd.Flags().DurationVar(&o.WaitTimeout, "wait-timeout", o.WaitTimeout, "Maximum duration to wait for the bastion to become available.")

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

	// create client for the garden cluster
	gardenClient, err := manager.GardenClient(currentTarget)
	if err != nil {
		return err
	}

	// fetch targeted shoot (ctx is cancellable to stop the keep alive goroutine later)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shoot, err := util.ShootForTarget(ctx, gardenClient, currentTarget)
	if err != nil {
		return err
	}

	// fetch the SSH key for the shoot nodes
	nodePrivateKey, err := getShootNodePrivateKey(ctx, gardenClient, shoot)
	if err != nil {
		return err
	}

	// save the key into a temporary file that we try to clean up when exiting
	nodePrivateKeyFile, err := savePublicKey(nodePrivateKey)
	if err != nil {
		return err
	}
	defer os.Remove(nodePrivateKeyFile)

	// if a node was given, check if the node exists
	// and if not, exit early and do not create a bastion
	node, err := getShootNode(ctx, o, manager, currentTarget)
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
	bastionID, err := utils.GenerateRandomString(8)
	if err != nil {
		return fmt.Errorf("failed to create bastion name: %v", err)
	}

	bastionName := fmt.Sprintf("cli-%s", strings.ToLower(bastionID))
	bastion := &operationsv1alpha1.Bastion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bastionName,
			Namespace: "garden-dev", // shoot.Namespace,
		},
		Spec: operationsv1alpha1.BastionSpec{
			ShootRef: corev1.LocalObjectReference{
				Name: shoot.Name,
			},
			SSHPublicKey: strings.TrimSpace(string(sshPublicKey)),
			Ingress:      policies,
		},
	}

	fmt.Fprintln(o.IOStreams.Out, "Creating bastion host…")

	if err := gardenClient.Create(ctx, bastion); err != nil {
		return fmt.Errorf("failed to create bastion: %v", err)
	}

	fmt.Fprintf(o.IOStreams.Out, "Waiting up to %v for bastion to be ready…", o.WaitTimeout)

	err = waitForBastion(ctx, o, gardenClient, bastion)

	fmt.Fprintln(o.IOStreams.Out, "")

	if err == wait.ErrWaitTimeout {
		fmt.Fprintln(o.IOStreams.Out, "Timed out waiting for the bastion to be ready.")
	} else if err != nil {
		fmt.Fprintf(o.IOStreams.Out, "An error occured while waiting: %v", err)
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

	// continuously keep the bastion alive by renewing its annotation
	go keepBastionAlive(ctx, gardenClient, bastion, o.IOStreams.ErrOut)

	if node != nil && o.Interactive {
		err = remoteShell(ctx, o, bastion, nodePrivateKeyFile, node)
	} else {
		err = waitForSignal(o, bastion, nodePrivateKeyFile, node)
	}

	fmt.Fprintln(o.IOStreams.Out, "Exiting…")

	// stop keeping the bastion alive
	cancel()
	<-ctx.Done()

	return err
}

func preferedBastionAddress(bastion *operationsv1alpha1.Bastion) string {
	if ingress := bastion.Status.Ingress; ingress != nil {
		if ingress.IP != "" {
			return ingress.IP
		}

		return ingress.Hostname
	}

	return ""
}

func waitForBastion(ctx context.Context, o *Options, gardenClient client.Client, bastion *operationsv1alpha1.Bastion) error {
	return wait.Poll(5*time.Second, o.WaitTimeout, func() (bool, error) {
		key := client.ObjectKeyFromObject(bastion)

		if err := gardenClient.Get(ctx, key, bastion); err != nil {
			return false, err
		}

		// TODO: update gardener dependency and use operationsv1alpha1.BastionReady const
		cond := corev1alpha1helper.GetCondition(bastion.Status.Conditions, "BastionReady")

		if cond == nil || cond.Status != v1alpha1.ConditionTrue {
			return false, nil
		}

		checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		if err := checkPortAvailable(checkCtx, preferedBastionAddress(bastion), SSHPort); err != nil {
			return false, nil
		}

		return true, nil
	})
}

// checkPortAvailable check whether the host with port is reachable within certain period of time
func checkPortAvailable(ctx context.Context, hostname string, port int) error {
	var dialer net.Dialer

	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(hostname, strconv.Itoa(port)))
	if conn != nil {
		conn.Close()
	}

	return err
}

func getShootNode(ctx context.Context, o *Options, manager target.Manager, currentTarget target.Target) (*corev1.Node, error) {
	if o.NodeName == "" {
		return nil, nil
	}

	shootClient, err := manager.ShootClusterClient(currentTarget)
	if err != nil {
		return nil, err
	}

	node := &corev1.Node{}
	if err := shootClient.Get(ctx, types.NamespacedName{Name: o.NodeName}, node); err != nil {
		return nil, err
	}

	return node, nil
}

func remoteShell(ctx context.Context, o *Options, bastion *operationsv1alpha1.Bastion, nodePrivateKeyFile string, node *corev1.Node) error {
	nodeHostname, err := getNodeHostname(node)
	if err != nil {
		return err
	}

	bastionAddr := preferedBastionAddress(bastion)
	connectCmd := sshCommandLine(o, bastionAddr, nodePrivateKeyFile, nodeHostname)

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
		"-i", nodePrivateKeyFile,
		fmt.Sprintf("%s@%s", SSHNodeUsername, nodeHostname),
	}

	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Stdout = o.IOStreams.Out
	cmd.Stdin = o.IOStreams.In
	cmd.Stderr = o.IOStreams.ErrOut

	return cmd.Run()
}

// waitForSignal informs the user about their options and keeps the
// bastion alive until gardenctl exits.
func waitForSignal(o *Options, bastion *operationsv1alpha1.Bastion, nodePrivateKeyFile string, node *corev1.Node) error {
	nodeHostname := "SHOOT_NODE"
	if node != nil {
		var err error

		nodeHostname, err = getNodeHostname(node)
		if err != nil {
			return err
		}
	}

	bastionAddr := preferedBastionAddress(bastion)
	connectCmd := sshCommandLine(o, bastionAddr, nodePrivateKeyFile, nodeHostname)

	fmt.Fprintln(o.IOStreams.Out, "Connect to Shoot nodes by using the bastion as a proxy/jump host, for example:")
	fmt.Fprintln(o.IOStreams.Out, "")
	fmt.Fprintln(o.IOStreams.Out, connectCmd)
	fmt.Fprintln(o.IOStreams.Out, "")
	fmt.Fprintln(o.IOStreams.Out, "Press Ctrl-C to stop gardenctl, after which the bastion will be removed.")

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan

	return nil
}

func sshCommandLine(o *Options, bastionAddr string, nodePrivateKeyFile string, nodeName string) string {
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

	connectCmd := fmt.Sprintf(
		`ssh -o "StrictHostKeyChecking=no" -o "IdentitiesOnly=yes" -i %s -o "ProxyCommand=%s" %s@%s`,
		nodePrivateKeyFile,
		proxyCmd,
		SSHNodeUsername,
		nodeName,
	)

	return connectCmd
}

func keepBastionAlive(ctx context.Context, gardenClient client.Client, bastion *operationsv1alpha1.Bastion, stderr io.Writer) {
	ticker := time.NewTicker(3 * time.Minute)
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
				// no return here, maybe on the next attempt it will work
			}

			// add the keepalive annotation
			oldBastion := bastion.DeepCopy()
			bastion.Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.GardenerOperationKeepalive
			if err := gardenClient.Patch(ctx, bastion, client.MergeFrom(oldBastion)); err != nil {
				fmt.Fprintf(stderr, "Failed to keep bastion alive: %v\n", err)
				// no return here, maybe on the next attempt it will work
			}
		}
	}
}

func getShootNodePrivateKey(ctx context.Context, gardenClient client.Client, shoot *gardencorev1beta1.Shoot) ([]byte, error) {
	secret := &corev1.Secret{}
	key := types.NamespacedName{
		Name:      fmt.Sprintf("%s.ssh-keypair", shoot.Name),
		Namespace: shoot.Namespace,
	}

	if err := gardenClient.Get(ctx, key, secret); err != nil {
		return nil, err
	}

	return secret.Data["id_rsa"], nil
}

func savePublicKey(key []byte) (string, error) {
	f, err := os.CreateTemp(os.TempDir(), "gctlv2*")
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
