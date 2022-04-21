## gardenctl kubeconfig

Print the kubeconfig for the current target

```
gardenctl kubeconfig [flags]
```

### Examples

```
# Print the kubeconfig for the current target 
gardenctl kubeconfig

# Print the kubeconfig for the current target in json format
gardenctl kubeconfig --output json

# Print the Shoot cluster kubeconfig for my-shoot
gardenctl kubeconfig --garden my-garden --project my-project --shoot my-shoot

# Print the Garden cluster kubeconfig of my-garden. The namespace of the project my-project is set as default
gardenctl kubeconfig --garden my-garden --project my-project
```

### Options

```
  -h, --help            help for kubeconfig
  -o, --output string   One of 'yaml' or 'json'.
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

