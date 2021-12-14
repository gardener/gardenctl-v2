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

run_test gardenctl-v2 "${SOURCE_PATH}" "${GO_TEST_ADDITIONAL_FLAGS}"
