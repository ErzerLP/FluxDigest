#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

bootstrap_miniflux() {
  [[ -n "${MINIFLUX_BASE_URL:-}" ]] || fail "MINIFLUX_BASE_URL is required"

  local existing_token="${APP_MINIFLUX_AUTH_TOKEN:-}"
  if [[ -n "${existing_token}" ]] && curl -fsS -H "X-Auth-Token: ${existing_token}" "${MINIFLUX_BASE_URL%/}/v1/me" >/dev/null; then
    printf '%s\n' "${existing_token}"
    return 0
  fi

  [[ -n "${MINIFLUX_ADMIN_USERNAME:-}" ]] || fail "MINIFLUX_ADMIN_USERNAME is required"
  [[ -n "${MINIFLUX_ADMIN_PASSWORD:-}" ]] || fail "MINIFLUX_ADMIN_PASSWORD is required"
  [[ -n "${MINIFLUX_API_KEY_DESCRIPTION:-}" ]] || fail "MINIFLUX_API_KEY_DESCRIPTION is required"

  local response token
  response="$(
    curl -fsS \
      -u "${MINIFLUX_ADMIN_USERNAME}:${MINIFLUX_ADMIN_PASSWORD}" \
      -H 'Content-Type: application/json' \
      -X POST \
      "${MINIFLUX_BASE_URL%/}/v1/api-keys" \
      -d "{\"description\":\"${MINIFLUX_API_KEY_DESCRIPTION}\"}"
  )"
  token="$(json_string_field "${response}" "token")"

  curl -fsS -H "X-Auth-Token: ${token}" "${MINIFLUX_BASE_URL%/}/v1/me" >/dev/null
  printf '%s\n' "${token}"
}
