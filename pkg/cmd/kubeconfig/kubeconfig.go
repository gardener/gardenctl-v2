/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package kubeconfig

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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
	o.AddFlags(cmd.Flags())

	utilruntime.Must(cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return o.PrintFlags.AllowedFormats(), cobra.ShellCompDirectiveNoFileComp
	}))

	return cmd
}

// options is a struct to support kubeconfig command
type options struct {
	base.Options

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

	// RawConfig holds the information needed to build connect to remote kubernetes clusters as a given user
	RawConfig clientcmdapi.Config
}

// newOptions returns initialized options
func newOptions(ioStreams util.IOStreams) *options {
	return &options{
		Options: base.Options{
			IOStreams: ioStreams,
		},
		PrintFlags: genericclioptions.NewPrintFlags("").WithTypeSetter(scheme.Scheme).WithDefaultOutput("yaml"),
	}
}

// AddFlags binds the command options to a given flagset.
func (o *options) AddFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&o.RawByteData, "raw", o.RawByteData, "Display raw byte data")
	flags.BoolVar(&o.Flatten, "flatten", o.Flatten, "Flatten the resulting kubeconfig file into self-contained output (useful for creating portable kubeconfig files)")
	flags.BoolVar(&o.Minify, "minify", o.Minify, "Remove all information not used by current-context from the output")
	flags.StringVar(&o.Context, "context", o.Context, "The name of the kubeconfig context to use")
}

// Complete adapts from the command line args to the data required.
func (o *options) Complete(f util.Factory, _ *cobra.Command, _ []string) error {
	manager, err := f.Manager()
	if err != nil {
		return err
	}

	currentTarget, err := manager.CurrentTarget()
	if err != nil {
		return err
	}

	if currentTarget.GardenName() == "" {
		return target.ErrNoGardenTargeted
	}

	printer, err := o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}

	o.PrintObject = printer.PrintObj

	ctx := f.Context()

	config, err := manager.ClientConfig(ctx, currentTarget)
	if err != nil {
		return err
	}

	rawConfig, err := config.RawConfig()
	if err != nil {
		return err
	}

	o.RawConfig = rawConfig

	return nil
}

// Run does the actual work of the command.
func (o *options) Run(f util.Factory) error {
	if o.Minify {
		if len(o.Context) > 0 {
			o.RawConfig.CurrentContext = o.Context
		}

		if err := clientcmdapi.MinifyConfig(&o.RawConfig); err != nil {
			return err
		}
	}

	if o.Flatten {
		if err := clientcmdapi.FlattenConfig(&o.RawConfig); err != nil {
			return err
		}
	} else if !o.RawByteData {
		clientcmdapi.ShortenConfig(&o.RawConfig)
	}

	convertedObj, err := clientcmdlatest.Scheme.ConvertToVersion(&o.RawConfig, clientcmdlatest.ExternalVersion)
	if err != nil {
		return err
	}

	// TODO align PrintObject in base and in this implementation
	return o.PrintObject(convertedObj, o.IOStreams.Out)
}
