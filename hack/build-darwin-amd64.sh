#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

# For the build step concourse will set the following environment variables:
# SOURCE_PATH - path to component repository root directory.
# BINARY_PATH - path to an existing (empty) directory to place build results into.

if [[ -z "${MAIN_REPO_DIR}" ]]; then
  export MAIN_REPO_DIR="$(readlink -f $(dirname ${0})/..)"
else
  export MAIN_REPO_DIR="$(readlink -f "${MAIN_REPO_DIR}")"
fi

if [[ -z "${BINARY_PATH}" ]]; then
  export BINARY_PATH="${MAIN_REPO_DIR}/bin"
else
  export BINARY_PATH="$(readlink -f "${BINARY_PATH}")"
fi

pushd "${MAIN_REPO_DIR}" > /dev/null

if [[ -z "${LD_FLAGS}" ]]; then
  export LD_FLAGS=$(./hack/get-build-ld-flags.sh)
fi
###############################################################################

out_file="${BINARY_PATH}"/darwin-amd64/garden-login_darwin_amd64

echo "building for darwin-amd64: ${out_file}"
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 GO111MODULE=on go build \
		-ldflags "${LD_FLAGS}" \
		-o "${out_file}" main.go

popd > /dev/null
