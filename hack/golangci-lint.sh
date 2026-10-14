#!/usr/bin/env bash

# SPDX-FileCopyrightText: Contributors to the Gardener project
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

source "$(dirname "${BASH_SOURCE[0]}")/setup-tools-env.sh"

if [ -z "$SOURCE_PATH" ]; then
  SOURCE_PATH="$(dirname "$0")/.."
fi
export SOURCE_PATH="$(readlink -f "$SOURCE_PATH")"


GOLANGCI_LINT_TIMEOUT="${GOLANGCI_LINT_TIMEOUT:-5m}"
GOLANGCI_LINT_VERBOSE="${GOLANGCI_LINT_VERBOSE:-0}"

VERBOSE_FLAG=""
if [ "${GOLANGCI_LINT_VERBOSE}" = "1" ]; then
  VERBOSE_FLAG="--verbose"
  echo "> Enabling verbose output for golangci-lint"
fi
echo "> Timeout for golangci-lint set to ${GOLANGCI_LINT_TIMEOUT}"

echo "> Running golangci-lint for $SOURCE_PATH"
pushd "$SOURCE_PATH" > /dev/null
golangci-lint run ${VERBOSE_FLAG} --timeout "${GOLANGCI_LINT_TIMEOUT}" ./...
popd > /dev/null