#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

check_mode() {
  local path="${1:?}"
  local expected="${2:?}"
  local actual
  actual="$(git -C "${ROOT}" ls-files --stage -- "${path}" | awk '{print $1}')"
  [[ -n "${actual}" ]] || {
    echo "missing git index entry: ${path}" >&2
    exit 1
  }
  [[ "${actual}" == "${expected}" ]] || {
    echo "unexpected git mode for ${path}: got ${actual}, want ${expected}" >&2
    exit 1
  }
}

check_mode "install.sh" "100755"
check_mode "deploy/stack/install.sh" "100755"
