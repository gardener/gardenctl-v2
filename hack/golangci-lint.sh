#!/usr/bin/env bash

# SPDX-FileCopyrightText: Contributors to the Gardener project
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

if [ -z "$SOURCE_PATH" ]; then
  SOURCE_PATH="$(dirname "$0")/.."
fi
export SOURCE_PATH="$(readlink -f "$SOURCE_PATH")"

# renovate: datasource=github-releases depName=golangci/golangci-lint
GOLANGCI_LINT_VERSION=v2.12.2

GOLANGCI_LINT_TIMEOUT="${GOLANGCI_LINT_TIMEOUT:-5m}"
GOLANGCI_LINT_VERBOSE="${GOLANGCI_LINT_VERBOSE:-0}"

GOLANGCI_LINT_BIN="$(go env GOPATH)/bin/golangci-lint"

# Install golangci-lint via `go install` if not present or version mismatch.
# `go install` verifies module integrity through the Go checksum database
# (sum.golang.org), eliminating the need to fetch and execute an unverified
# remote installer script.
if [ ! -x "$GOLANGCI_LINT_BIN" ] || ! "$GOLANGCI_LINT_BIN" --version | grep -q "version ${GOLANGCI_LINT_VERSION#v}"; then
  echo "> Installing golangci-lint ${GOLANGCI_LINT_VERSION} via go install..."
  go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${GOLANGCI_LINT_VERSION}"
fi

VERBOSE_FLAG=""
if [ "${GOLANGCI_LINT_VERBOSE}" = "1" ]; then
  VERBOSE_FLAG="--verbose"
  echo "> Enabling verbose output for golangci-lint"
fi
echo "> Timeout for golangci-lint set to ${GOLANGCI_LINT_TIMEOUT}"

echo "> Running golangci-lint for $SOURCE_PATH"
pushd "$SOURCE_PATH" > /dev/null
"$GOLANGCI_LINT_BIN" run ${VERBOSE_FLAG} --timeout "${GOLANGCI_LINT_TIMEOUT}" ./...
popd > /dev/null
