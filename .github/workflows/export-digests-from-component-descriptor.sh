#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${1:-}" ]]; then
  echo "Usage: ${BASH_SOURCE[0]} COMPONENT_DESCRIPTOR" >&2
  exit 1
fi

command -v yq >/dev/null 2>&1 || { echo "yq is needed" >&2; exit 1; }

component_descriptor="$1"

read_digest() {
  local os="$1" arch="$2" ref
  ref="$(yq -r "
    .component.resources[]
    | select(.name == \"gardenctl-v2\" and .extraIdentity.os == \"$os\" and .extraIdentity.architecture == \"$arch\")
    | .access.localReference
  " "$component_descriptor")" || return 1

  [[ -n "$ref" && "$ref" != "null" ]] || return 1
  printf '%s\n' "${ref#sha256:}"
}

DARWIN_SHA_AMD64="$(read_digest darwin amd64)"
: "${DARWIN_SHA_AMD64:?failed to resolve digest for darwin/amd64}"
DARWIN_SHA_ARM64="$(read_digest darwin arm64)"
: "${DARWIN_SHA_ARM64:?failed to resolve digest for darwin/arm64}"
LINUX_SHA_AMD64="$(read_digest linux amd64)"
: "${LINUX_SHA_AMD64:?failed to resolve digest for linux/amd64}"
LINUX_SHA_ARM64="$(read_digest linux arm64)"
: "${LINUX_SHA_ARM64:?failed to resolve digest for linux/arm64}"
WINDOWS_SHA="$(read_digest windows amd64)"
: "${WINDOWS_SHA:?failed to resolve digest for windows/amd64}"

printf 'darwin-amd64: %s\n' "$DARWIN_SHA_AMD64"
printf 'darwin-arm64: %s\n' "$DARWIN_SHA_ARM64"
printf 'linux-amd64: %s\n' "$LINUX_SHA_AMD64"
printf 'linux-arm64: %s\n' "$LINUX_SHA_ARM64"
printf 'windows-amd64: %s\n' "$WINDOWS_SHA"

export DARWIN_SHA_AMD64 DARWIN_SHA_ARM64 LINUX_SHA_AMD64 LINUX_SHA_ARM64 WINDOWS_SHA
