#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
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
  export LD_FLAGS=$("${MAIN_REPO_DIR}/hack/get-build-ld-flags.sh")
fi
###############################################################################

out_file="${BINARY_PATH}/${GOOS}-${GOARCH}/gardenctl_v2_${GOOS}_${GOARCH}"

if [[ "${GOOS}" == "windows" ]]; then
  out_file="${out_file}.exe"
fi

echo "building for ${GOOS}-${GOARCH}: ${out_file}"
CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} GO111MODULE=on go build \
		-ldflags "${LD_FLAGS}" \
		-o "${out_file}" main.go

popd > /dev/null
