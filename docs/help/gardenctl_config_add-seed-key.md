## gardenctl config add-seed-key

Add a trusted seed kubeconfig signing key for a Garden

### Synopsis

Add the PEM-encoded ECDSA P-256 public key from your landscape administrator to a configured Garden by identity or alias.
Run 'gardenctl config view' to find the Garden's identity or alias.
Use --file for a PEM file or --key for the full PEM block with line breaks.
Adding the same key again succeeds without changing the configuration.

Add a key only after confirming with the landscape administrator that it is theirs.
The holder of the matching private key decides which kubeconfig gardenctl loads for the Garden's non-managed seeds,
including the commands that kubeconfig runs on your machine to authenticate.

```
gardenctl config add-seed-key GARDEN (--file PUBLIC_KEY.pem | --key PEM) [flags]
```

### Examples

```
gardenctl config add-seed-key landscape-dev --file public-key.pem

# Replace the placeholder with the actual public key and keep the line breaks.
gardenctl config add-seed-key landscape-dev --key '-----BEGIN PUBLIC KEY-----
<public key from your landscape administrator>
-----END PUBLIC KEY-----'
```

### Options

```
  -f, --file string         Path to the PEM public key from your landscape administrator
  -h, --help[=true|false]   help for add-seed-key
      --key string          Full PEM public key, including line breaks
```

### Options inherited from parent commands

```
      --add-dir-header                      If true, adds the file directory to the header of the log messages
      --alsologtostderr                     log to standard error as well as files (no effect when -logtostderr=true)
      --alsologtostderrthreshold severity   logs at or above this threshold go to stderr when -alsologtostderr=true (no effect when -logtostderr=true)
      --config string                       config file (default is ~/.garden/gardenctl-v2.yaml)
      --legacy-stderr-threshold-behavior    If true, stderrthreshold is ignored when logtostderr=true (legacy behavior). If false, stderrthreshold is honored even when logtostderr=true (default true)
      --log-backtrace-at traceLocation      when logging hits line file:N, emit a stack trace (default :0)
      --log-dir string                      If non-empty, write log files in this directory (no effect when -logtostderr=true)
      --log-file string                     If non-empty, use this log file (no effect when -logtostderr=true)
      --log-file-max-size uint              Defines the maximum size a log file can grow to (no effect when -logtostderr=true). Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --logtostderr                         log to standard error instead of files (default true)
      --one-output                          If true, only write logs to their native severity level (vs also writing to each lower severity level; no effect when -logtostderr=true)
      --skip-headers                        If true, avoid header prefixes in the log messages
      --skip-log-headers                    If true, avoid headers when opening log files (no effect when -logtostderr=true)
      --stderrthreshold severity            logs at or above this threshold go to stderr when writing to files and stderr (no effect when -logtostderr=true or -alsologtostderr=true unless -legacy_stderr_threshold_behavior=false) (default 2)
  -v, --v Level                             number for the log level verbosity
      --vmodule moduleSpec                  comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO

* [gardenctl config](gardenctl_config.md)	 - Modify gardenctl configuration file using subcommands

