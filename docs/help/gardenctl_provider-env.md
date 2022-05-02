## gardenctl provider-env

Generate the cloud provider CLI configuration script for the specified shell

### Synopsis

Generate the cloud provider CLI configuration script for the specified shell.
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

### Options

```
  -f, --force   Generate the script even if there are access restrictions to be confirmed
  -h, --help    help for provider-env
  -u, --unset   Generate the script to unset the cloud provider CLI environment variables and logout for 
```

### Options inherited from parent commands

```
      --add-dir-header                   If true, adds the file directory to the header of the log messages
      --alsologtostderr                  log to standard error as well as files
      --config string                    config file (default is ~/.garden/gardenctl-v2.yaml)
      --control-plane                    target control plane of shoot, use together with shoot argument
      --garden string                    target the given garden cluster
      --log-backtrace-at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log-dir string                   If non-empty, write log files in this directory
      --log-file string                  If non-empty, use this log file
      --log-file-max-size uint           Defines the maximum size a log file can grow to. Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --logtostderr                      log to standard error instead of files (default true)
      --one-output                       If true, only write logs to their native severity level (vs also writing to each lower severity level)
      --project string                   target the given project
      --seed string                      target the given seed cluster
      --shoot string                     target the given shoot cluster
      --skip-headers                     If true, avoid header prefixes in the log messages
      --skip-log-headers                 If true, avoid headers when opening log files
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
  -v, --v Level                          number for the log level verbosity
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO

* [gardenctl](gardenctl.md)	 - Gardenctl is a utility to interact with Gardener installations
* [gardenctl provider-env bash](gardenctl_provider-env_bash.md)	 - Generate the cloud provider CLI configuration script for bash
* [gardenctl provider-env fish](gardenctl_provider-env_fish.md)	 - Generate the cloud provider CLI configuration script for fish
* [gardenctl provider-env powershell](gardenctl_provider-env_powershell.md)	 - Generate the cloud provider CLI configuration script for powershell
* [gardenctl provider-env zsh](gardenctl_provider-env_zsh.md)	 - Generate the cloud provider CLI configuration script for zsh

