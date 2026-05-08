# gardenctl-v2

![](logo/logo_gardener_cli_large.png)

[![CI Build status](https://github.com/gardener/gardenctl-v2/actions/workflows/non-release.yaml/badge.svg?branch=master)](https://github.com/gardener/gardenctl-v2/actions/workflows/non-release.yaml?query=branch%3Amaster)
[![REUSE status](https://api.reuse.software/badge/github.com/gardener/gardenctl-v2)](https://api.reuse.software/info/github.com/gardener/gardenctl-v2)
[![Go Report Card](https://goreportcard.com/badge/github.com/gardener/gardenctl-v2)](https://goreportcard.com/report/github.com/gardener/gardenctl-v2)
[![release](https://badge.fury.io/gh/gardener%2Fgardenctl-v2.svg)](https://badge.fury.io/gh/gardener%2Fgardenctl-v2)

## What is `gardenctl`?

gardenctl is a command-line client for the Gardener. It facilitates the administration of one or many garden, seed and shoot clusters. Use this tool to configure access to clusters and configure cloud provider CLI tools. It also provides support for accessing cluster nodes via ssh.

## Installation

Install the latest release from [Homebrew](https://brew.sh/), [winget](https://learn.microsoft.com/en-us/windows/package-manager/winget/), [Chocolatey](https://chocolatey.org/packages/gardenctl-v2) or [GitHub Releases](https://github.com/gardener/gardenctl-v2/releases).

### Install using Package Managers

```sh
# Homebrew (macOS and Linux)
brew install gardener/tap/gardenctl-v2

# winget (Windows)
winget install -e --id Gardener.gardenctl

# Chocolatey (Windows)
# default location C:\ProgramData\chocolatey\bin\gardenctl-v2.exe
choco install gardenctl-v2
```

Attention `brew` users: `gardenctl-v2` uses the same binary name as the legacy `gardenctl` (`gardener/gardenctl`) CLI. If you have an existing installation you should remove it with `brew uninstall gardenctl` before attempting to install `gardenctl-v2`. Alternatively, you can choose to link the binary using a different name. If you try to install without removing or relinking the old installation, brew will run into an error and provide instructions how to resolve it.

### Install from Github Release

If you install via GitHub releases, you need to

- put the `gardenctl` binary on your path
- and [install gardenlogin](https://github.com/gardener/gardenlogin#installation).

The other install methods do this for you.

```bash
# Example for macOS

# set operating system and architecture
os=darwin # choose between darwin, linux, windows
arch=arm64 # choose between amd64, arm64

# Get latest version. Alternatively set your desired version
version=$(curl -s https://raw.githubusercontent.com/gardener/gardenctl-v2/master/LATEST)

# Download gardenctl
curl -LO "https://github.com/gardener/gardenctl-v2/releases/download/${version}/gardenctl_v2_${os}_${arch}"

# Make the gardenctl binary executable
chmod +x "./gardenctl_v2_${os}_${arch}"

# Move the binary in to your PATH
sudo mv "./gardenctl_v2_${os}_${arch}" /usr/local/bin/gardenctl
```

## Configuration

`gardenctl` requires a configuration file. The default location is in `~/.garden/gardenctl-v2.yaml`.

You can modify this file directly using the `gardenctl config` command. It allows adding, modifying and deleting gardens.

Example `config` command:

```bash
# Adapt the path to your kubeconfig file for the garden cluster (not to be mistaken with your shoot cluster)
export KUBECONFIG=~/relative/path/to/kubeconfig.yaml

# Fetch cluster-identity of garden cluster from the configmap
cluster_identity=$(kubectl -n kube-system get configmap cluster-identity -ojsonpath={.data.cluster-identity})

# Configure garden cluster
gardenctl config set-garden $cluster_identity --kubeconfig $KUBECONFIG
```

This command will create or update a garden with the provided identity and kubeconfig path of your garden cluster.

### Example Config

```yaml
gardens:
  - identity: landscape-dev # Unique identity of the garden cluster. See cluster-identity ConfigMap in kube-system namespace of the garden cluster
    kubeconfig: ~/relative/path/to/kubeconfig.yaml
# name: my-name # An alternative, unique garden name for targeting
# context: different-context # Overrides the current-context of the garden cluster kubeconfig
# patterns: ~ # List of regex patterns for pattern targeting
# defaultKubeconfigAccessLevel: # Default access level requested per target scope
#   shoots: viewer              # admin (default) | viewer | auto
#   managedSeeds: admin         # admin (default) | viewer | auto
```

> [!NOTE]
> - To use the kubeconfigs for **shoot clusters** provided by `gardenctl`, you need to have [gardenlogin](https://github.com/gardener/gardenlogin) installed as a `kubectl` auth plugin.
> - If your **garden cluster** kubeconfig uses OIDC authentication, ensure that you have the [kubelogin](https://github.com/int128/kubelogin) `kubectl` auth plugin installed.

### Kubeconfig Access Level

For shoots and managed seeds, `gardenctl` requests a kubeconfig from `gardenlogin` with a configurable access level:

- `admin` (default) - full cluster-admin credentials.
- `viewer` - read-only credentials with no access to encrypted resources (e.g. Secrets).
- `auto` - try admin first; fall back to viewer if admin is denied. Convenient when you don't know your RBAC up front; for users who *do* have admin, this is effectively the same as `admin`.

Set per-scope defaults per garden under `defaultKubeconfigAccessLevel`. The `shoots` and `managedSeeds` fields are independent - for example, an admin who wants least-privilege access to customer shoots while still keeping admin access for seed-level debugging would set:

```yaml
defaultKubeconfigAccessLevel:
  shoots: viewer
  managedSeeds: admin
```

Override per invocation with the `--kubeconfig-access-level` flag, available on commands that produce a kubeconfig (`target`, `kubeconfig`):

```bash
gardenctl kubeconfig --kubeconfig-access-level=admin            # one-off escalation
gardenctl target --shoot my-shoot --kubeconfig-access-level=viewer
```

The same defaults can be set via the CLI rather than hand-editing the YAML:

```bash
gardenctl config set-garden prd-garden \
  --default-shoot-access-level viewer \
  --default-managed-seed-access-level admin
```

The access level only applies to **shoots and managed seeds**, where Gardener provides admin/viewer kubeconfig subresources that gardenlogin requests on demand. It does not apply to:

- The **garden cluster itself** - gardenctl uses the user's own kubeconfig file as configured. There is no API to "downgrade" those credentials; if you want viewer-only access to the garden, point gardenctl at a kubeconfig whose underlying RBAC is read-only.
- **Non-managed seeds** - credentials come from a static `<name>.login` Secret in the garden cluster, which only contains admin credentials. Requesting `viewer` or `auto` for a non-managed seed (via flag or the `managedSeeds` default) returns an error.

> [!NOTE]
> `auto` returns admin whenever your RBAC allows it. If you want guaranteed read-only access, configure `viewer` explicitly.

#### Verifying which access level is in use

When `gardenctl` explicitly chose an access level (via flag or per-garden config), it surfaces it inline after `target` and on stderr after `kubeconfig`:

```console
$ gardenctl target shoot prod-cluster --kubeconfig-access-level=viewer
Successfully targeted shoot "prod-cluster" (access level: viewer)
```

When neither a flag nor a per-garden config is set, `gardenctl` stays silent and lets `gardenlogin`'s own default apply (currently `auto`).

For an explicit check, the access level is also embedded as a `--access-level=…` argument in the produced kubeconfig's exec plugin section (omitted when `gardenctl` deferred to `gardenlogin`'s default):

```bash
gardenctl kubeconfig | grep -- '--access-level='
```

### Config Path Overwrite

- The `gardenctl` config path can be overwritten with the environment variable `GCTL_HOME`.
- The `gardenctl` config name can be overwritten with the environment variable `GCTL_CONFIG_NAME`.

```bash
export GCTL_HOME=/alternate/garden/config/dir
export GCTL_CONFIG_NAME=myconfig # without extension!
# config is expected to be under /alternate/garden/config/dir/myconfig.yaml
```

### Recommended Shell Configuration

The easiest way to configure your shell for gardenctl is using `gardenctl rc`.
For bash, add the following to your `~/.bashrc`:

```bash
source <(gardenctl rc bash)
```

Other shells (`zsh`, `fish`, `powershell`) are also supported. Run `gardenctl rc [shell] --help` for details.

This automatically:
- Sets the `GCTL_SESSION_ID` and `KUBECONFIG` environment variables, preventing the warning about a missing `KUBECONFIG` when running `gardenctl target`
- Loads shell completion
- Provides useful aliases (prefixed with `g` by default)

### Shell Session

> [!NOTE]
> If you source `gardenctl rc` as described above, this is already handled for you.

The state of gardenctl is bound to a shell session and is not shared across windows, tabs or panes.
A shell session is defined by the environment variable `GCTL_SESSION_ID`. If this is not defined,
the value of the `TERM_SESSION_ID` environment variable is used instead. If both are not defined,
this leads to an error and gardenctl cannot be executed. The `target.yaml` and temporary
`kubeconfig.*.yaml` files are stored in the following directory `${TMPDIR}/garden/${GCTL_SESSION_ID}`.

You can make sure that `GCTL_SESSION_ID` or `TERM_SESSION_ID` is always present by adding
the following code to your terminal profile `~/.profile`, `~/.bashrc` or comparable file.

```sh
bash and zsh: [ -n "$GCTL_SESSION_ID" ] || [ -n "$TERM_SESSION_ID" ] || export GCTL_SESSION_ID=$(uuidgen)
```

```sh
fish:         [ -n "$GCTL_SESSION_ID" ] || [ -n "$TERM_SESSION_ID" ] || set -gx GCTL_SESSION_ID (uuidgen)
```

```ps
powershell:   if ( !(Test-Path Env:GCTL_SESSION_ID) -and !(Test-Path Env:TERM_SESSION_ID) ) { $Env:GCTL_SESSION_ID = [guid]::NewGuid().ToString() }
```

### Completion

> [!NOTE]
> If you source `gardenctl rc` as described above, this is already handled for you.

Gardenctl supports completion that will help you working with the CLI and save you typing effort.
It will also help you find clusters by providing suggestions for gardener resources such as shoots or projects.
Completion is supported for `bash`, `zsh`, `fish` and `powershell`.
You will find more information on how to configure your shell completion for gardenctl by executing the help for
your shell completion command. Example:

```bash
gardenctl completion bash --help
```

## Usage

### Targeting

You can set a target to use it in subsequent commands. You can also overwrite the target for each command individually.

Note that this will not affect your KUBECONFIG env variable. To update the KUBECONFIG env for your current target see [Configure KUBECONFIG](#configure-kubeconfig-for-shoot-clusters) section

Example:

```bash
# target control plane
gardenctl target --garden landscape-dev --project my-project --shoot my-shoot --control-plane
```

Find more information in the [documentation](docs/usage/targeting.md).

### Configure KUBECONFIG for Shoot Clusters

Generate a script that points KUBECONFIG to the targeted cluster for the specified shell. Use together with `eval` to configure your shell. Example for `bash`:

```bash
eval "$(gardenctl kubectl-env bash)"
```

To load the kubectl configuration for each bash session add the command at the end of the `~/.bashrc` file.

### Configure Cloud Provider CLIs

See the dedicated documentation for usage, options, and OpenStack validation requirements: [Configure Cloud Provider CLIs (provider-env)](docs/usage/provider-env.md)

### SSH

Establish an SSH connection to a Shoot cluster's node.

```bash
gardenctl ssh my-node
```
