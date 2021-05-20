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

# Install golangci-lint (linting tool)
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.39.0

cd "$SOURCE_PATH"

golangci-lint run ./... -E golint,whitespace,wsl --skip-files "zz_generated.*"  --verbose --timeout 2m
