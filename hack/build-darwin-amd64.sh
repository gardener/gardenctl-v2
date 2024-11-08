#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

# For the build step concourse will set the following environment variables:
# MAIN_REPO_DIR - path to component repository root directory.

if [[ -z "${MAIN_REPO_DIR}" ]]; then
  export MAIN_REPO_DIR="$(readlink -f $(dirname ${0})/..)"
else
  export MAIN_REPO_DIR="$(readlink -f "${MAIN_REPO_DIR}")"
fi

export GOOS=darwin
export GOARCH=amd64

${MAIN_REPO_DIR}/hack/build.sh
