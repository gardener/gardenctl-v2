## gardenctl target control-plane

Target the control plane of the shoot

### Synopsis

Target the control plane of the shoot cluster to set the scope for the next operations

```
gardenctl target control-plane [flags]
```

### Examples

```
# target control-plane of currently selected shoot cluster
gardenctl target control-plane

# target control-plane of shoot my-shoot
gardenctl target control-plane --garden my-garden --project my-project --shoot my-shoot
```

### Options

```
      --garden string    target the given garden cluster
  -h, --help             help for control-plane
      --project string   target the given project
      --seed string      target the given seed cluster
      --shoot string     target the given shoot cluster
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

* [gardenctl target](gardenctl_target.md)	 - Set scope for next operations, using subcommands or pattern

