#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

# MAIN_REPO_DIR - path to component repository root directory.
# BINARY_PATH - path to an existing (empty) directory to place build results into.

export MAIN_REPO_DIR="$(readlink -f ${BASH_SOURCE[0]}/..)"

pushd "${MAIN_REPO_DIR}" > /dev/null

if [[ -z "${LD_FLAGS}" ]]; then
  export LD_FLAGS=$("${MAIN_REPO_DIR}/hack/get-build-ld-flags.sh")
fi
###############################################################################

if [[ -z "${out_file:-}" ]]; then
  out_file="${MAIN_REPO_DIR}/${GOOS}-${GOARCH}/gardenctl_v2_${GOOS}_${GOARCH}"
  if [[ "${GOOS}" == "windows" ]]; then
    out_file="${out_file}.exe"
  fi
fi

echo "building for ${GOOS}-${GOARCH}: ${out_file}"
CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} GO111MODULE=on go build \
		-ldflags "${LD_FLAGS}" \
		-o "${out_file}" main.go

popd > /dev/null
