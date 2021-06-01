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
	"net/http"
	"strings"

	"github.com/gardener/gardenctl-v2/internal/util"

	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewCommand returns a new ssh command.
func NewCommand(f util.Factory, o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "Establish an SSH connection to a Shoot cluster's node",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(f, cmd, args); err != nil {
				return fmt.Errorf("failed to complete command options: %w", err)
			}
			if err := o.Validate(); err != nil {
				return err
			}

			return runCommand(f, o)
		},
	}

	cmd.Flags().StringArrayVar(&o.CIDRs, "cidr", nil, "CIDRs to allow access to the Bastion host; if not given, the host's public IP is auto-detected.")
	cmd.Flags().StringVar(&o.SSHPublicKeyFile, "public-key-file", "", "Path to the file that contains your public SSH key.")

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

	// fetch targeted shoot
	ctx := context.Background()

	shoot, err := util.ShootForTarget(ctx, gardenClient, currentTarget)
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

	bastion := &operationsv1alpha1.Bastion{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "cli-",
			Namespace:    shoot.Namespace,
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

	fmt.Fprintln(o.IOStreams.Out, "Waiting for bastion to be ready…")

	return nil
}

func getPublicIP(ctx context.Context) (string, error) {
	req, err := http.NewRequest("GET", "https://api.ipify.org/", nil)
	if err != nil {
		return "", err
	}

	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	ip, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	ipAddress := strings.TrimSpace(string(ip))

	if net.ParseIP(ipAddress) == nil {
		return "", fmt.Errorf("API returned an invalid IP (%q)", ipAddress)
	}

	return ipAddress, nil
}
