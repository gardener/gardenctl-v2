/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package cloudenv

import (
	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
)

// NewCmdCloudEnv returns a new cloudenv command.
func NewCmdCloudEnv(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := &cmdOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
	runE := base.WrapRunE(o, f)
	cmd := &cobra.Command{
		Use:   "cloud-env",
		Short: "generate the cloud provider CLI configuration script for the specified shell",
		Long: `Generate the cloud provider CLI configuration script for the specified shell.
See each sub-command's help for details on how to use the generated script.

The generated script sets the environment variables for the cloud provider CLI of the targeted shoot.
In addition, the Azure CLI requires to sign in with a service principal and the gcloud CLI requires to activate a service-account.
Since these actions are persistent, they should be undone when closing the shell or changing the target using the --unset flag.

The CLI of a corresponding cloud provider must be installed.
Please refer to the installation instructions of the respective provider:
* Amazon Web Services (aws) - https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html
* Microsoft Azure (az) - https://docs.microsoft.com/cli/azure/install-azure-cli
* Google cloud (gcloud) - https://cloud.google.com/sdk/docs/install
* Openstack (openstack) - https://docs.openstack.org/newton/user-guide/common/cli-install-openstack-command-line-clients.html
* Alibaba cloud (aliyun) - alicloud - https://www.alibabacloud.com/help/product/29991.htm
* Hetzner cloud (hcloud) - https://community.hetzner.com/tutorials/howto-hcloud-cli

To extend this list of cloud providers place a template for the new provider in the "templates" folder
of the gardenctl home directory ($GCTL_HOME or $HOME/.garden).
Please refer to the templates of the already supported cloud providers which can be found
here https://github.com/gardener/gardenctl-v2/tree/master/pkg/cmd/cloudenv/templates.`,
		Aliases: []string{"provider-env"},
	}
	o.AddFlags(cmd.PersistentFlags())

	for _, s := range validShells {
		s.AddCommand(cmd, runE)
	}

	return cmd
}
