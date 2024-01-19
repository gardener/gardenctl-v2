# gardenctl-v2

![](logo/logo_gardener_cli_large.png)

[![Go Report Card](https://goreportcard.com/badge/github.com/gardener/gardenctl-v2)](https://goreportcard.com/report/github.com/gardener/gardenctl-v2)
[![release](https://badge.fury.io/gh/gardener%2Fgardenctl-v2.svg)](https://badge.fury.io/gh/gardener%2Fgardenctl-v2)
[![reuse compliant](https://reuse.software/badge/reuse-compliant.svg)](https://reuse.software/)

## What is `gardenctl`?

gardenctl is a command-line client for the Gardener. It facilitates the administration of one or many garden, seed and shoot clusters. Use this tool to configure access to clusters and configure cloud provider CLI tools. It also provides support for accessing cluster nodes via ssh.

## Installation

Install the latest release from [Homebrew](https://brew.sh/), [Chocolatey](https://chocolatey.org/packages/gardenctl-v2) or [GitHub Releases](https://github.com/gardener/gardenctl-v2/releases).

### Install using Package Managers

```sh
# Homebrew (macOS and Linux)
brew install gardener/tap/gardenctl-v2

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
arch=amd64 # choose between amd64, arm64

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
```

Note: You need to have [gardenlogin](https://github.com/gardener/gardenlogin) installed as `kubectl` plugin in order to use the `kubeconfig`s for `Shoot` clusters provided by `gardenctl`.

### Config Path Overwrite

- The `gardenctl` config path can be overwritten with the environment variable `GCTL_HOME`.
- The `gardenctl` config name can be overwritten with the environment variable `GCTL_CONFIG_NAME`.

```bash
export GCTL_HOME=/alternate/garden/config/dir
export GCTL_CONFIG_NAME=myconfig # without extension!
# config is expected to be under /alternate/garden/config/dir/myconfig.yaml
```

### Shell Session

The state of gardenctl is bound to a shell session and is not shared across windows, tabs or panes.
A shell session is defined by the environment variable `GCTL_SESSION_ID`. If this is not defined,
the value of the `TERM_SESSION_ID` environment variable is used instead. If both are not defined,
this leads to an error and gardenctl cannot be executed. The `target.yaml` and temporary
`kubeconfig.*.yaml` files are store in the following directory `${TMPDIR}/garden/${GCTL_SESSION_ID}`.

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
eval $(gardenctl kubectl-env bash)
```

### Configure Cloud Provider CLIs

Generate the cloud provider CLI configuration script for the specified shell. Use together with `eval` to configure your shell. Example for `bash`:

```bash
eval $(gardenctl provider-env bash)
```

### SSH

Establish an SSH connection to a Shoot cluster's node.

```bash
gardenctl ssh my-node
```
