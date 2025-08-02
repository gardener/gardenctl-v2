#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

# For the verify step concourse will set the following environment variables:
# MAIN_REPO_DIR - path to component repository root directory.

if [[ -z "${MAIN_REPO_DIR}" ]]; then
  export MAIN_REPO_DIR="$(readlink -f "$(dirname ${0})/..")"
else
  export MAIN_REPO_DIR="$(readlink -f ${MAIN_REPO_DIR})"
fi

# renovate: datasource=github-releases depName=golangci/golangci-lint
golangci_lint_version=v2.3.1

GOLANGCI_LINT_ADDITIONAL_FLAGS=${GOLANGCI_LINT_ADDITIONAL_FLAGS:-""}

# Install golangci-lint (linting tool)
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin "$golangci_lint_version"

cd "$MAIN_REPO_DIR"

echo '> Run golangci-lint'

golangci-lint -v run ./... ${GOLANGCI_LINT_ADDITIONAL_FLAGS}
