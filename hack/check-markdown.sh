#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
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

echo '> Check Markdown'

tmpDir=$(mktemp -d)
function cleanup {
  rm -rf "$tmpDir"
}
trap cleanup EXIT ERR INT TERM

pushd "$MAIN_REPO_DIR" > /dev/null

export OUT_DIR=$tmpDir
go run "internal/gen/markdown.go"

EXIT_CODE=0
output=$(diff -x '.DS_Store' "$MAIN_REPO_DIR/docs/help" "$OUT_DIR") || EXIT_CODE=$?
if [[ ${EXIT_CODE} -gt 0 ]]; then
  echo 'Error: Diff does not match. Run "make gen-markdown" and commit the generated files'
  echo 'Cause:'
  echo "$output"
  exit 1
fi

echo 'Markdown is up-to-date'

popd > /dev/null
