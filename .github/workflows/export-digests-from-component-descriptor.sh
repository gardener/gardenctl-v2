#!/usr/bin/env bash

set -euo pipefail

if [ -z "${1:-}" ]; then
  echo "Usage: ${BASH_SOURCE[0]} COMPONENT_DESCRIPTOR"
  exit 1
fi

if ! which yq &>/dev/null; then
  echo "yq is needed"
  exit 1
fi

component_descriptor="${1}"

resources=$( \
  cat "${component_descriptor}" | \
  yq '.component.resources[] | select (.name == "gardenctl-v2" )' \
)

read_digest() {
  os=${1}
  arch=${2}
  digest=$( \
    echo "${resources}" | \
    jq -r "select(.extraIdentity.os == \"${os}\" and .extraIdentity.architecture == \"${arch}\") | .access.localReference"
  )

  # strip `sha256:`-prefix
  echo "${digest}" | cut -d: -f2
}

export DARWIN_SHA_AMD64=$(read_digest darwin amd64)
export DARWIN_SHA_ARM64=$(read_digest darwin arm64)
export LINUX_SHA_AMD64=$(read_digest linux amd64)
export LINUX_SHA_ARM64=$(read_digest linux arm64)
export WINDOWS_SHA=$(read_digest windows amd64)
