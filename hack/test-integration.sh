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

GO_TEST_ADDITIONAL_FLAGS=${GO_TEST_ADDITIONAL_FLAGS:-""}

OS=${OS:-$(go env GOOS)}
ARCH=${ARCH:-$(go env GOARCH)}

function run_test {
  local component=$1
  local target_dir=$2
  local go_test_additional_flags=$3
  echo "> Test $component"

  pushd "$target_dir"

  GO111MODULE=on go test ./... ${go_test_additional_flags} -coverprofile cover.out

  popd
}

run_test gardenctl-v2 "${MAIN_REPO_DIR}" "${GO_TEST_ADDITIONAL_FLAGS}"
