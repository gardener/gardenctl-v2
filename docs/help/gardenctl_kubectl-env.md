## gardenctl kubectl-env

Generate a script that points KUBECONFIG to the targeted cluster for the specified shell

### Synopsis

Generate a script that points KUBECONFIG to the targeted cluster for the specified shell.
See each sub-command's help for details on how to use the generated script.

The generated script points the KUBECONFIG environment variable to the currently targeted shoot, seed or garden cluster.


### Options

```
      --control-plane    override current target with shoot control plane
      --garden string    override current target with the given garden cluster
  -h, --help             help for kubectl-env
      --project string   override current target with the given project
      --seed string      override current target with the given seed cluster
      --shoot string     override current target with the given shoot cluster
  -u, --unset            Generate the script to unset the KUBECONFIG environment variable for 
```

### Options inherited from parent commands

```
      --add-dir-header                   If true, adds the file directory to the header of the log messages
      --alsologtostderr                  log to standard error as well as files
      --config string                    config file (default is ~/.garden/gardenctl-v2.yaml)
      --log-backtrace-at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log-dir string                   If non-empty, write log files in this directory
      --log-file string                  If non-empty, use this log file
      --log-file-max-size uint           Defines the maximum size a log file can grow to. Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --logtostderr                      log to standard error instead of files (default true)
      --one-output                       If true, only write logs to their native severity level (vs also writing to each lower severity level)
      --skip-headers                     If true, avoid header prefixes in the log messages
      --skip-log-headers                 If true, avoid headers when opening log files
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
  -v, --v Level                          number for the log level verbosity
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO

* [gardenctl](gardenctl.md)	 - Gardenctl is a utility to interact with Gardener installations
* [gardenctl kubectl-env bash](gardenctl_kubectl-env_bash.md)	 - Generate a script that points KUBECONFIG to the targeted cluster for bash
* [gardenctl kubectl-env fish](gardenctl_kubectl-env_fish.md)	 - Generate a script that points KUBECONFIG to the targeted cluster for fish
* [gardenctl kubectl-env powershell](gardenctl_kubectl-env_powershell.md)	 - Generate a script that points KUBECONFIG to the targeted cluster for powershell
* [gardenctl kubectl-env zsh](gardenctl_kubectl-env_zsh.md)	 - Generate a script that points KUBECONFIG to the targeted cluster for zsh

