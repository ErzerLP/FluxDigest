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

  local response token status body
  response="$(
    curl -sS \
      -u "${MINIFLUX_ADMIN_USERNAME}:${MINIFLUX_ADMIN_PASSWORD}" \
      -H 'Content-Type: application/json' \
      -X POST \
      -w $'\n%{http_code}' \
      "${MINIFLUX_BASE_URL%/}/v1/api-keys" \
      -d "{\"description\":\"${MINIFLUX_API_KEY_DESCRIPTION}\"}"
  )"
  body="${response%$'\n'*}"
  status="${response##*$'\n'}"
  if [[ ! "${status}" =~ ^2[0-9][0-9]$ ]]; then
    fail "创建 Miniflux API Key 失败: status=${status:-unknown} body=${body:-empty}"
  fi

  token="$(json_string_field "${body}" "token")"

  curl -fsS -H "X-Auth-Token: ${token}" "${MINIFLUX_BASE_URL%/}/v1/me" >/dev/null
  printf '%s\n' "${token}"
}
