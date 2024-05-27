#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

# For the check step concourse will set the following environment variables:
# SOURCE_PATH - path to component repository root directory.

if [[ -z "${SOURCE_PATH}" ]]; then
  export SOURCE_PATH="$(readlink -f "$(dirname ${0})/..")"
else
  export SOURCE_PATH="$(readlink -f ${SOURCE_PATH})"
fi

# renovate: datasource=github-releases depName=golangci/golangci-lint
golangci_lint_version=v1.59.0

# Install golangci-lint (linting tool)
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin "$golangci_lint_version"

cd "$SOURCE_PATH"

echo '> Run golangci-lint'

golangci-lint -v run ./...
