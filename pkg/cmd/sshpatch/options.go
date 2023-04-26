/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package sshpatch

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/spf13/cobra"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/klog/v2"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/cmd/ssh"
)

type options struct {
	base.Options
	ssh.AccessConfig

	// Bastion is the Bastion corresponding to the provided BastionName
	Bastion *operationsv1alpha1.Bastion

	// bastionPatcher lists bastions created by the current user
	bastionPatcher bastionPatcher
}

func newOptions(ioStreams util.IOStreams) *options {
	return &options{
		AccessConfig: ssh.AccessConfig{},
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
}

func (o *options) patchBastionIngress(ctx context.Context) error {
	logger := klog.FromContext(ctx)

	var policies []operationsv1alpha1.BastionIngressPolicy

	oldBastion := o.Bastion.DeepCopy()

	for _, cidr := range o.CIDRs {
		if *o.Bastion.Spec.ProviderType == "gcp" {
			ip, _, err := net.ParseCIDR(cidr)
			if err != nil {
				return err
			}

			if ip.To4() == nil {
				if !o.AutoDetected {
					return fmt.Errorf("GCP only supports IPv4: %s", cidr)
				}

				logger.Info("GCP only supports IPv4, skipped CIDR", "cidr", cidr)

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
		return errors.New("no ingress policies could be created")
	}

	o.Bastion.Spec.Ingress = policies

	err := o.bastionPatcher.Patch(ctx, o.Bastion, oldBastion)
	if err != nil {
		return fmt.Errorf("failed to patch bastion ingress: %w", err)
	}

	return nil
}

func (o *options) Run(f util.Factory) error {
	ctx, cancel := context.WithTimeout(f.Context(), 30*time.Second)
	defer cancel()

	if err := o.patchBastionIngress(ctx); err != nil {
		return err
	}

	fmt.Fprintf(o.IOStreams.Out, "Successfully patched bastion %q\n", o.Bastion.Name)

	return nil
}

func (o *options) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(f.Context(), 30*time.Second)
	defer cancel()

	logger := klog.FromContext(ctx)

	manager, err := f.Manager()
	if err != nil {
		return err
	}

	bastionListPatcher, err := newBastionListPatcher(manager)
	if err != nil {
		return fmt.Errorf("could not create bastion lister: %w", err)
	}

	o.bastionPatcher = bastionListPatcher

	if err := o.AccessConfig.Complete(f, cmd, args); err != nil {
		return err
	}

	bastions, err := bastionListPatcher.List(ctx)
	if err != nil {
		return err
	}

	if len(bastions) == 0 {
		return errors.New("no bastions found for current user")
	}

	if len(args) == 0 {
		if len(bastions) > 1 {
			return fmt.Errorf("multiple bastions were found and the target bastion needs to be explicitly defined")
		}

		o.Bastion = &bastions[0]

		age := f.Clock().Now().Sub(o.Bastion.CreationTimestamp.Time).Round(time.Second).String()
		logger.Info("Auto-selected bastion", "bastion", klog.KObj(o.Bastion), "age", age, "shoot", klog.KRef(o.Bastion.Namespace, o.Bastion.Spec.ShootRef.Name))
	} else {
		bastionName := args[0]

		for _, b := range bastions {
			if b.Name == bastionName {
				o.Bastion = &b
				break
			}
		}

		if o.Bastion == nil {
			return fmt.Errorf("bastion %q for current user not found", o.Bastion.Name)
		}
	}

	return nil
}

func (o *options) Validate() error {
	if err := o.Options.Validate(); err != nil {
		return err
	}

	if err := o.AccessConfig.Validate(); err != nil {
		return err
	}

	if o.Bastion == nil {
		return fmt.Errorf("bastion is required")
	}

	return nil
}
