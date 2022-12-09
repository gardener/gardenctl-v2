/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package env

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/flags"
)

// NewCmdProviderEnv returns a new provider-env command.
func NewCmdProviderEnv(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := &options{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
	runE := base.WrapRunE(o, f)
	cmd := &cobra.Command{
		Use:   "provider-env",
		Short: "Generate the cloud provider CLI configuration script for the specified shell",
		Long: `Generate the cloud provider CLI configuration script for the specified shell.
See each sub-command's help for details on how to use the generated script.

The generated script sets the environment variables for the cloud provider CLI of the targeted shoot.
In addition, the Azure CLI requires to sign in with a service principal and the gcloud CLI requires to activate a service-account.
Thereby the configuration location of the corresponding cloud provider CLI is pointed to a temporary folder in the
session directory, so that the standard configuration files in the user's home folder are not affected.
By using the --unset flag you can force a logout or revoke the service-account.

The CLI of a corresponding cloud provider must be installed.
Please refer to the installation instructions of the respective provider:
* Amazon Web Services (aws) - https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html
* Microsoft Azure (az) - https://docs.microsoft.com/cli/azure/install-azure-cli
* Google cloud (gcloud) - https://cloud.google.com/sdk/docs/install
* Openstack (openstack) - https://docs.openstack.org/newton/user-guide/common/cli-install-openstack-command-line-clients.html
* Alibaba cloud (aliyun) - alicloud - https://www.alibabacloud.com/help/product/29991.htm
* Hetzner cloud (hcloud) - https://community.hetzner.com/tutorials/howto-hcloud-cli

To overwrite the default templates or add support for custom (out of tree) cloud providers place a template
for the respective provider in the "templates" folder of the gardenctl home directory ($GCTL_HOME or $HOME/.garden).
Please refer to the templates of the already supported cloud providers which can be found
here https://github.com/gardener/gardenctl-v2/tree/master/pkg/cmd/env/templates.`,
		Aliases: []string{"p-env", "cloud-env"},
	}

	persistentFlags := cmd.PersistentFlags()
	o.AddFlags(persistentFlags)

	manager, err := f.Manager()
	utilruntime.Must(err)
	manager.TargetFlags().AddFlags(persistentFlags)
	flags.RegisterTargetFlagCompletionFuncs(cmd, f, ioStreams, persistentFlags)

	for _, s := range validShells {
		cmd.AddCommand(&cobra.Command{
			Use:   string(s),
			Short: fmt.Sprintf("Generate the cloud provider CLI configuration script for %s", s),
			Long: fmt.Sprintf("Generate the cloud provider CLI configuration script for %s.\n\n"+
				"To load the cloud provider CLI configuration script in your current shell session:\n%s\n",
				s, s.Prompt(runtime.GOOS)+s.EvalCommand(fmt.Sprintf("gardenctl %s %s", cmd.Name(), s)),
			),
			RunE: runE,
		})
	}

	return cmd
}

// NewCmdKubectlEnv returns a new kubectl-env command.
func NewCmdKubectlEnv(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := &options{
		Options: base.Options{
			IOStreams: ioStreams,
		},
		ProviderType: "kubernetes",
	}
	runE := base.WrapRunE(o, f)
	cmd := &cobra.Command{
		Use:   "kubectl-env",
		Short: "Generate a script that points KUBECONFIG to the targeted cluster for the specified shell",
		Long: `Generate a script that points KUBECONFIG to the targeted cluster for the specified shell.
See each sub-command's help for details on how to use the generated script.

The generated script points the KUBECONFIG environment variable to the currently targeted shoot, seed or garden cluster.
`,
		Aliases: []string{"k-env", "cluster-env"},
	}
	o.AddFlags(cmd.PersistentFlags())

	for _, s := range validShells {
		cmd.AddCommand(&cobra.Command{
			Use:   string(s),
			Short: fmt.Sprintf("Generate a script that points KUBECONFIG to the targeted cluster for %s", s),
			Long: fmt.Sprintf("Generate a script that points KUBECONFIG to the targeted cluster for %s.\n\n"+
				"To load the kubectl configuration script in your current shell session:\n%s\n",
				s, s.Prompt(runtime.GOOS)+s.EvalCommand(fmt.Sprintf("gardenctl %s %s", cmd.Name(), s)),
			),
			RunE: runE,
		})
	}

	return cmd
}
