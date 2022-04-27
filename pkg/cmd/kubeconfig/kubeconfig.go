/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package kubeconfig

import (
	"github.com/spf13/cobra"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/kubernetes/scheme"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

// NewCmdKubeconfig returns a new kubeconfig command.
func NewCmdKubeconfig(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := newOptions(ioStreams)

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

	o.PrintFlags.AddFlags(cmd)

	cmd.Flags().BoolVar(&o.RawByteData, "raw", o.RawByteData, "Display raw byte data")
	cmd.Flags().BoolVar(&o.Flatten, "flatten", o.Flatten, "Flatten the resulting kubeconfig file into self-contained output (useful for creating portable kubeconfig files)")
	cmd.Flags().BoolVar(&o.Minify, "minify", o.Minify, "Remove all information not used by current-context from the output")
	cmd.Flags().StringVar(&o.Context, "context", o.Context, "The name of the kubeconfig context to use")

	utilruntime.Must(cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return o.PrintFlags.AllowedFormats(), cobra.ShellCompDirectiveNoFileComp
	}))

	return cmd
}

// kubeconfigOptions is a struct to support kubeconfig command
type kubeconfigOptions struct {
	base.Options

	// CurrentTarget is the current target
	CurrentTarget target.Target

	// PrintFlags composes common printer flag structs
	PrintFlags *genericclioptions.PrintFlags
	// ResourcePrinterFunc is a function that can print objects
	PrintObject printers.ResourcePrinterFunc

	// Flatten flag the resulting kubeconfig file into self-contained output
	Flatten bool
	// Minify flag to remove all information not used by current-context from the output
	Minify bool
	// RawByteData flag to display raw byte data
	RawByteData bool

	// Context holds the name of the kubeconfig context to use
	Context string
}

// newOptions returns initialized kubeconfigOptions
func newOptions(ioStreams util.IOStreams) *kubeconfigOptions {
	return &kubeconfigOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
		PrintFlags: genericclioptions.NewPrintFlags("").WithTypeSetter(scheme.Scheme).WithDefaultOutput("yaml"),
	}
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

	printer, err := o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}

	o.PrintObject = printer.PrintObj

	return nil
}

// Validate validates the provided command kubeconfigOptions.
func (o *kubeconfigOptions) Validate() error {
	if o.CurrentTarget.GardenName() == "" {
		return target.ErrNoGardenTargeted
	}

	return nil
}

// Run does the actual work of the command.
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

	rawConfig, err := config.RawConfig()
	if err != nil {
		return err
	}

	if o.Minify {
		if len(o.Context) > 0 {
			rawConfig.CurrentContext = o.Context
		}

		if err := clientcmdapi.MinifyConfig(&rawConfig); err != nil {
			return err
		}
	}

	if o.Flatten {
		if err := clientcmdapi.FlattenConfig(&rawConfig); err != nil {
			return err
		}
	} else if !o.RawByteData {
		clientcmdapi.ShortenConfig(&rawConfig)
	}

	convertedObj, err := clientcmdlatest.Scheme.ConvertToVersion(&rawConfig, clientcmdlatest.ExternalVersion)
	if err != nil {
		return err
	}

	return o.PrintObject(convertedObj, o.IOStreams.Out)
}
