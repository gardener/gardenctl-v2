## gardenctl kubectl-env zsh

Generate a script that points KUBECONFIG to the targeted cluster for zsh

### Synopsis

Generate a script that points KUBECONFIG to the targeted cluster for zsh.

To load the kubectl configuration script in your current shell session:
$ eval "$(gardenctl kubectl-env zsh)"

To apply this setting automatically in every shell session, consider adding the command at the end of your ~/.zshrc file.


```
gardenctl kubectl-env zsh [flags]
```

### Options

```
  -h, --help   help for zsh
```

### Options inherited from parent commands

```
      --add-dir-header                   If true, adds the file directory to the header of the log messages
      --alsologtostderr                  log to standard error as well as files (no effect when -logtostderr=true)
      --config string                    config file (default is ~/.garden/gardenctl-v2.yaml)
      --log-backtrace-at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log-dir string                   If non-empty, write log files in this directory (no effect when -logtostderr=true)
      --log-file string                  If non-empty, use this log file (no effect when -logtostderr=true)
      --log-file-max-size uint           Defines the maximum size a log file can grow to (no effect when -logtostderr=true). Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --logtostderr                      log to standard error instead of files (default true)
      --one-output                       If true, only write logs to their native severity level (vs also writing to each lower severity level; no effect when -logtostderr=true)
      --skip-headers                     If true, avoid header prefixes in the log messages
      --skip-log-headers                 If true, avoid headers when opening log files (no effect when -logtostderr=true)
      --stderrthreshold severity         logs at or above this threshold go to stderr when writing to files and stderr (no effect when -logtostderr=true or -alsologtostderr=true) (default 2)
  -u, --unset                            Generate the script to unset the KUBECONFIG environment variable for 
  -v, --v Level                          number for the log level verbosity
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO

* [gardenctl kubectl-env](gardenctl_kubectl-env.md)	 - Generate a script that points KUBECONFIG to the targeted cluster for the specified shell

