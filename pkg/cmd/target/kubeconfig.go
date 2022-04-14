/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

// NewCmdKubeconfig returns a new target kubeconfig command.
func NewCmdKubeconfig(f util.Factory, o *KubeconfigOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubeconfig",
		Short: "Print the kubeconfig for the current target",
		RunE:  base.WrapRunE(o, f),
	}

	o.AddFlags(cmd.Flags())

	return cmd
}

// KubeconfigOptions is a struct to support kubeconfig command
type KubeconfigOptions struct {
	base.Options

	// CurrentTarget is the current target
	CurrentTarget target.Target
}

// NewKubeconfigOptions returns initialized KubeconfigOptions
func NewKubeconfigOptions(ioStreams util.IOStreams) *KubeconfigOptions {
	return &KubeconfigOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
}

// Complete adapts from the command line args to the data required.
func (o *KubeconfigOptions) Complete(f util.Factory, _ *cobra.Command, _ []string) error {
	manager, err := f.Manager()
	if err != nil {
		return err
	}

	o.CurrentTarget, err = manager.CurrentTarget()
	if err != nil {
		return err
	}

	return nil
}

// Validate validates the provided command options.
func (o *KubeconfigOptions) Validate() error {
	if o.CurrentTarget.GardenName() == "" {
		return target.ErrNoGardenTargeted
	}

	return nil
}

func (o *KubeconfigOptions) Run(f util.Factory) error {
	ctx := f.Context()

	manager, err := f.Manager()
	if err != nil {
		return err
	}

	config, err := manager.ClientConfig(ctx, o.CurrentTarget)
	if err != nil {
		return err
	}

	b, err := writeRawConfig(config)
	if err != nil {
		return err
	}

	_, err = o.IOStreams.Out.Write(b)

	return err
}

func writeRawConfig(config clientcmd.ClientConfig) ([]byte, error) {
	rawConfig, err := config.RawConfig()
	if err != nil {
		return nil, err
	}

	return clientcmd.Write(rawConfig)
}
