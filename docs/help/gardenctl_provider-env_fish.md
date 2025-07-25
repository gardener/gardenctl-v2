## gardenctl provider-env fish

Generate the cloud provider CLI configuration script for fish

### Synopsis

Generate the cloud provider CLI configuration script for fish.

To load the cloud provider CLI configuration script in your current shell session:
$ eval (gardenctl provider-env fish)


```
gardenctl provider-env fish [flags]
```

### Options

```
  -h, --help   help for fish
```

### Options inherited from parent commands

```
      --add-dir-header                   If true, adds the file directory to the header of the log messages
      --alsologtostderr                  log to standard error as well as files (no effect when -logtostderr=true)
      --config string                    config file (default is ~/.garden/gardenctl-v2.yaml)
  -y, --confirm-access-restriction       Confirm any access restrictions. Set this flag only if you are completely aware of the access restrictions.
      --control-plane                    target control plane of shoot, use together with shoot argument
  -f, --force                            Deprecated. Use --confirm-access-restriction instead. Generate the script even if there are access restrictions to be confirmed.
      --garden string                    target the given garden cluster
      --gcp-allowed-patterns strings     Additional allowed patterns for GCP service account fields, in the format 'field=value', e.g., 'universe_domain=googleapis.com'. These are merged with defaults and configuration.
      --log-backtrace-at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log-dir string                   If non-empty, write log files in this directory (no effect when -logtostderr=true)
      --log-file string                  If non-empty, use this log file (no effect when -logtostderr=true)
      --log-file-max-size uint           Defines the maximum size a log file can grow to (no effect when -logtostderr=true). Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --logtostderr                      log to standard error instead of files (default true)
      --one-output                       If true, only write logs to their native severity level (vs also writing to each lower severity level; no effect when -logtostderr=true)
      --project string                   target the given project
      --seed string                      target the given seed cluster
      --shoot string                     target the given shoot cluster
      --skip-headers                     If true, avoid header prefixes in the log messages
      --skip-log-headers                 If true, avoid headers when opening log files (no effect when -logtostderr=true)
      --stderrthreshold severity         logs at or above this threshold go to stderr when writing to files and stderr (no effect when -logtostderr=true or -alsologtostderr=true) (default 2)
  -u, --unset                            Generate the script to unset the cloud provider CLI environment variables and logout for 
  -v, --v Level                          number for the log level verbosity
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO

* [gardenctl provider-env](gardenctl_provider-env.md)	 - Generate the cloud provider CLI configuration script for the specified shell

