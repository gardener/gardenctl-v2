# Configure Cloud Provider CLIs with provider-env

The `provider-env` command generates a shell script to configure cloud provider CLIs (aws, az, gcloud, openstack, aliyun, hcloud) for the currently targeted Shoot. Evaluate the generated script in your shell to export the required environment variables and perform any necessary provider CLI setup.

## Basic usage

Generate and evaluate the script for your shell. Example for `bash`:

```bash
eval "$(gardenctl provider-env bash)"
```

Alternatively, you can generate the script for other shells using subcommands: `bash`, `zsh`, `fish`, `powershell`.

The generated script:
- Sets provider-specific environment variables for the targeted Shoot
- Uses a temporary session directory so your default CLI configs are not modified
- Supports `--unset` to clean up and log out/revoke credentials

Ensure the respective provider CLI is installed on your system. See the in-command help for links.

## Overriding templates or adding custom providers

To override templates or add support for an out-of-tree provider, place a template file under the `templates` directory in your gardenctl home (`$GCTL_HOME` or `$HOME/.garden`). Use existing templates under `pkg/env/templates/` as reference.

## OpenStack: Allowed `authURL` patterns (required)

> [!NOTE]
> - For Shoots with provider type `openstack`, `gardenctl` validates the `authURL` from the credentials against a list of allowed patterns.
> - You must configure these allowed patterns; otherwise `provider-env` will fail with a validation error for OpenStack.
> - There are no built-in defaults because OpenStack auth endpoints are installation-specific.

You can configure allowed patterns via the gardenctl configuration file or via command-line flags.

### Configure via gardenctl config

Add patterns under `provider.openstack.allowedPatterns` in your `gardenctl` config file (default: `~/.garden/gardenctl-v2.yaml`). Use either a full `uri`, or a `host` with optional `path`/`port`, or restrict `scheme`.

Example (full URI):

```yaml
provider:
  openstack:
    allowedPatterns:
    - field: authURL
      uri: https://keystone.example.com:5000/v3
```

### Configure via command-line flags

You can augment configuration with flags when running `provider-env`:

- `--openstack-allowed-uri-patterns` allows a simplified `field=uri` syntax. Example:

  ```bash
  gardenctl provider-env bash \
    --openstack-allowed-uri-patterns="authURL=https://keystone.example.com:5000/v3"
  ```
