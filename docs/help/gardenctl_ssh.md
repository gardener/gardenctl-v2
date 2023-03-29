## gardenctl ssh

Establish an SSH connection to a Shoot cluster's node

```
gardenctl ssh [NODE_NAME] [flags]
```

### Options

```
      --cidr stringArray          CIDRs to allow access to the bastion host; if not given, your system's public IPs (v4 and v6) are auto-detected.
      --control-plane             target control plane of shoot, use together with shoot argument
      --garden string             target the given garden cluster
  -h, --help                      help for ssh
      --interactive               Open an SSH connection instead of just providing the bastion host (only if NODE_NAME is provided). (default true)
      --keep-bastion              Do not delete immediately when gardenctl exits (Bastions will be garbage-collected after some time)
      --no-keepalive              Exit after the bastion host became available without keeping the bastion alive or establishing an SSH connection. Note that this flag requires the flags --interactive=false and --keep-bastion to be set
  -o, --output string             One of 'yaml' or 'json'.
      --project string            target the given project
      --public-key-file string    Path to the file that contains a public SSH key. If not given, a temporary keypair will be generated.
      --seed string               target the given seed cluster
      --shoot string              target the given shoot cluster
      --skip-availability-check   Skip checking for SSH bastion host availability.
      --wait-timeout duration     Maximum duration to wait for the bastion to become available. (default 10m0s)
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
      --stderrthreshold severity         logs at or above this threshold go to stderr when writing to files and stderr (no effect when -logtostderr=true or -alsologtostderr=false) (default 2)
  -v, --v Level                          number for the log level verbosity
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO

* [gardenctl](gardenctl.md)	 - Gardenctl is a utility to interact with Gardener installations

