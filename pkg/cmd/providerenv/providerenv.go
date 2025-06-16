/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package providerenv

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/env"
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
here https://github.com/gardener/gardenctl-v2/tree/master/pkg/cmd/env/templates.

For shoots of provider type gcp (Google cloud), certain fields in the service account credential configuration are validated against allowed patterns. Each allowed pattern is a string in the format "field_name=allowed_value", where "allowed_value" is the exact value that the field must match.

The default allowed patterns are:
- "universe_domain=googleapis.com"
- "token_uri=https://accounts.google.com/o/oauth2/token"
- "token_uri=https://oauth2.googleapis.com/token"
- "auth_uri=https://accounts.google.com/o/oauth2/auth"
- "auth_provider_x509_cert_url=https://www.googleapis.com/oauth2/v1/certs"
- "client_x509_cert_url=https://www.googleapis.com/robot/v1/metadata/x509/{encoded_client_email}"

For the "client_x509_cert_url" field, the "{encoded_client_email}" placeholder in the allowed value is replaced with the URL-encoded "client_email" from the credential configuration before comparison.

You can extend these allowed patterns by:
- Adding them to the gardenctl configuration file under the "provider.gcp.allowedPatterns" key as a list of strings. For example:


    provider:
      gcp:
        allowedPatterns:
        - universe_domain=example.com
        - token_uri=https://example.com/token
        - client_x509_cert_url=https://example.com/{encoded_client_email}

- Using the "--gcp-allowed-patterns" command-line flag, providing additional patterns, e.g., --gcp-allowed-patterns "token_uri=https://example.com/token".
`,
		Aliases: []string{"p-env", "cloud-env"},
		RunE:    runE,
	}

	persistentFlags := cmd.PersistentFlags()
	o.AddFlags(persistentFlags)

	f.TargetFlags().AddFlags(persistentFlags)
	flags.RegisterCompletionFuncsForTargetFlags(cmd, f, ioStreams, persistentFlags)

	// add output flag only to the base provider-env command
	cmdFlags := cmd.Flags()
	o.Options.AddFlags(cmdFlags)

	for _, s := range env.ValidShells() {
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
