#!/usr/bin/env bash

# SPDX-FileCopyrightText: Contributors to the Gardener project
#
# SPDX-License-Identifier: Apache-2.0

# This script sets up the TOOLS_BIN_DIR and prepends it to PATH.
# Source this script from other hack scripts to ensure the correct
# tool binaries are found regardless of whether the script is invoked
# via `make` or directly.
#
# Usage: source "$(dirname "${BASH_SOURCE[0]}")/setup-tools-env.sh"

if [ -z "$TOOLS_BIN_DIR" ]; then
  REPO_ROOT="$(readlink -f "$(dirname "${BASH_SOURCE[0]}")/..")"
  SYSTEM_NAME="$(uname -s | tr '[:upper:]' '[:lower:]')"
  SYSTEM_ARCH="$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')"
  TOOLS_BIN_DIR="${REPO_ROOT}/hack/tools/bin/${SYSTEM_NAME}-${SYSTEM_ARCH}"
  echo "Dir: $TOOLS_BIN_DIR"
  export TOOLS_BIN_DIR
fi

# Prepend TOOLS_BIN_DIR to PATH if not already present
case ":${PATH}:" in
  *":${TOOLS_BIN_DIR}:"*) ;;
  *) export PATH="${TOOLS_BIN_DIR}:${PATH}" ;;
esac
