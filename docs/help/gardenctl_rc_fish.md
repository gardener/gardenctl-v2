## gardenctl rc fish

Generate a gardenctl startup script for fish

### Synopsis

Generate a gardenctl startup script for fish that contains various tweaks,
such as setting environment variables, loading completions and adding some helpful aliases or functions.

To load gardenctl startup script for each fish session add the following line at the end of the ~/.config/fish/config.fish file:

    gardenctl rc fish | source

You will need to start a new shell for this setup to take effect.


```
gardenctl rc fish [flags]
```

### Options

```
  -h, --help            help for fish
      --no-completion   The startup script should not setup completion
  -p, --prefix string   The prefix used for aliases and functions (default "g")
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

* [gardenctl rc](gardenctl_rc.md)	 - Generate a gardenctl startup script for the specified shell

