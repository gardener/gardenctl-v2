#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

if [ -z "$SOURCE_PATH" ]; then
  SOURCE_PATH="$(dirname "$0")/.."
fi
export SOURCE_PATH="$(readlink -f "$SOURCE_PATH")"

# renovate: datasource=github-releases depName=golangci/golangci-lint
GOLANGCI_LINT_VERSION=v2.7.2

GOLANGCI_LINT_TIMEOUT="${GOLANGCI_LINT_TIMEOUT:-5m}"
GOLANGCI_LINT_VERBOSE="${GOLANGCI_LINT_VERBOSE:-0}"

# Install golangci-lint if not present or version mismatch
if ! which golangci-lint >/dev/null 2>&1 || ! golangci-lint --version | grep -q "version ${GOLANGCI_LINT_VERSION#v}"; then
  echo "> Downloading golangci-lint..."
  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin ${GOLANGCI_LINT_VERSION}
fi

VERBOSE_FLAG=""
if [ "${GOLANGCI_LINT_VERBOSE}" = "1" ]; then
  VERBOSE_FLAG="--verbose"
  echo "> Enabling verbose output for golangci-lint"
fi
echo "> Timeout for golangci-lint set to ${GOLANGCI_LINT_TIMEOUT}"

echo "> Running golangci-lint for $SOURCE_PATH"
pushd "$SOURCE_PATH" > /dev/null
"$(go env GOPATH)"/bin/golangci-lint run ${VERBOSE_FLAG} --timeout "${GOLANGCI_LINT_TIMEOUT}" ./...
popd > /dev/null
