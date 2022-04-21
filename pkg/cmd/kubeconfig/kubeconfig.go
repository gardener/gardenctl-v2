/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package kubeconfig

import (
	"fmt"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

// NewCmdKubeconfig returns a new target kubeconfig command.
func NewCmdKubeconfig(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := &kubeconfigOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}

	cmd := &cobra.Command{
		Use:   "kubeconfig",
		Short: "Print the kubeconfig for the current target",
		Example: `# Print the kubeconfig for the current target 
gardenctl kubeconfig

# Print the kubeconfig for the current target in json format
gardenctl kubeconfig --output json

# Print the Shoot cluster kubeconfig for my-shoot
gardenctl kubeconfig --garden my-garden --project my-project --shoot my-shoot

# Print the Garden cluster kubeconfig of my-garden. The namespace of the project my-project is set as default
gardenctl kubeconfig --garden my-garden --project my-project`,
		RunE: base.WrapRunE(o, f),
	}

	o.AddFlags(cmd.Flags())

	return cmd
}

// kubeconfigOptions is a struct to support kubeconfig command
type kubeconfigOptions struct {
	base.Options

	// CurrentTarget is the current target
	CurrentTarget target.Target
}

// Complete adapts from the command line args to the data required.
func (o *kubeconfigOptions) Complete(f util.Factory, _ *cobra.Command, _ []string) error {
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
func (o *kubeconfigOptions) Validate() error {
	if o.CurrentTarget.GardenName() == "" {
		return target.ErrNoGardenTargeted
	}

	return nil
}

func (o *kubeconfigOptions) Run(f util.Factory) error {
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

	_, err = fmt.Fprintf(o.IOStreams.Out, "%s", b)

	return err
}

func writeRawConfig(config clientcmd.ClientConfig) ([]byte, error) {
	rawConfig, err := config.RawConfig()
	if err != nil {
		return nil, err
	}

	return clientcmd.Write(rawConfig)
}
