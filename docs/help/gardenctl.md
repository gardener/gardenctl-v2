## gardenctl

Gardenctl is a utility to interact with Gardener installations

### Synopsis

Gardenctl is a utility to interact with Gardener installations.

The state of gardenctl is bound to a shell session and is not shared across windows, tabs or panes.
A shell session is defined by the environment variable GCTL_SESSION_ID. If this is not defined,
the value of the TERM_SESSION_ID environment variable is used instead. If both are not defined,
this leads to an error and gardenctl cannot be executed. The target.yaml and temporary
kubeconfig.*.yaml files are stored in the following directory ${TMPDIR}/garden/${GCTL_SESSION_ID}.

You can make sure that GCTL_SESSION_ID or TERM_SESSION_ID is always present by adding
the following code to your terminal profile ~/.profile, ~/.bashrc or comparable file.
  bash and zsh:

      [ -n "$GCTL_SESSION_ID" ] || [ -n "$TERM_SESSION_ID" ] || export GCTL_SESSION_ID=$(uuidgen)

  fish:

      [ -n "$GCTL_SESSION_ID" ] || [ -n "$TERM_SESSION_ID" ] || set -gx GCTL_SESSION_ID (uuidgen)

  powershell:

      if ( !(Test-Path Env:GCTL_SESSION_ID) -and !(Test-Path Env:TERM_SESSION_ID) ) { $Env:GCTL_SESSION_ID = [guid]::NewGuid().ToString() }

Find more information at: https://github.com/gardener/gardenctl-v2/blob/master/README.md


### Options

```
      --add-dir-header                   If true, adds the file directory to the header of the log messages
      --alsologtostderr                  log to standard error as well as files (no effect when -logtostderr=true)
      --config string                    config file (default is ~/.garden/gardenctl-v2.yaml)
  -h, --help                             help for gardenctl
      --log-backtrace-at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log-dir string                   If non-empty, write log files in this directory (no effect when -logtostderr=true)
      --log-file string                  If non-empty, use this log file (no effect when -logtostderr=true)
      --log-file-max-size uint           Defines the maximum size a log file can grow to (no effect when -logtostderr=true). Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --logtostderr                      log to standard error instead of files (default true)
      --one-output                       If true, only write logs to their native severity level (vs also writing to each lower severity level; no effect when -logtostderr=true)
      --skip-headers                     If true, avoid header prefixes in the log messages
      --skip-log-headers                 If true, avoid headers when opening log files (no effect when -logtostderr=true)
      --stderrthreshold severity         logs at or above this threshold go to stderr when writing to files and stderr (no effect when -logtostderr=true or -alsologtostderr=true) (default 2)
  -v, --v Level                          number for the log level verbosity
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO

* [gardenctl config](gardenctl_config.md)	 - Modify gardenctl configuration file using subcommands
* [gardenctl kubeconfig](gardenctl_kubeconfig.md)	 - Print the kubeconfig for the current target
* [gardenctl kubectl-env](gardenctl_kubectl-env.md)	 - Generate a script that points KUBECONFIG to the targeted cluster for the specified shell
* [gardenctl provider-env](gardenctl_provider-env.md)	 - Generate the cloud provider CLI configuration script for the specified shell
* [gardenctl rc](gardenctl_rc.md)	 - Generate a gardenctl startup script for the specified shell
* [gardenctl resolve](gardenctl_resolve.md)	 - Resolve the current target
* [gardenctl ssh](gardenctl_ssh.md)	 - Establish an SSH connection to a node of a Shoot cluster
* [gardenctl ssh-patch](gardenctl_ssh-patch.md)	 - Update a bastion host previously created through the ssh command
* [gardenctl target](gardenctl_target.md)	 - Set scope for next operations, using subcommands or pattern
* [gardenctl version](gardenctl_version.md)	 - Print the gardenctl version information

