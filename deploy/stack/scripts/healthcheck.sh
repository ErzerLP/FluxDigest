#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

wait_for_http_ok() {
  local url="${1:?url is required}"
  local attempts="${2:-30}"
  local sleep_seconds="${3:-2}"

  local i
  for ((i = 1; i <= attempts; i++)); do
    if curl -fsS -o /dev/null "${url}"; then
      log_info "HTTP healthcheck OK: ${url}"
      return 0
    fi
    log_warn "HTTP healthcheck not ready (${i}/${attempts}): ${url}"
    sleep "${sleep_seconds}"
  done

  log_error "HTTP healthcheck failed after ${attempts} attempts: ${url}"
  return 1
}