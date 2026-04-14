#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

preferred_python_cmd() {
  if command -v python >/dev/null 2>&1 && python --version >/dev/null 2>&1; then
    printf '%s\n' "python"
    return 0
  fi
  if command -v python3 >/dev/null 2>&1 && python3 --version >/dev/null 2>&1; then
    printf '%s\n' "python3"
    return 0
  fi
  return 1
}

miniflux_api_key_token_from_list() {
  local payload="${1:?}"
  local description="${2:?}"
  local python_cmd
  if python_cmd="$(preferred_python_cmd)"; then
    MINIFLUX_API_KEYS_PAYLOAD="${payload}" "${python_cmd}" - "${description}" <<'PY'
import json
import os
import sys

description = sys.argv[1]
payload = os.environ.get("MINIFLUX_API_KEYS_PAYLOAD", "")

try:
    data = json.loads(payload)
except Exception:
    sys.exit(1)

for item in data:
    if not isinstance(item, dict):
        continue
    if item.get("description") != description:
        continue
    token = item.get("token") or ""
    if token:
        print(token)
        sys.exit(0)

sys.exit(1)
PY
    return $?
  fi

  local compact escaped_description token
  compact="$(printf '%s' "${payload}" | tr -d '\r\n')"
  escaped_description="$(
    printf '%s' "${description}" |
      sed -e 's/[.[\*^$()+?{|]/\\&/g' -e 's/\//\\\//g'
  )"
  token="$(
    printf '%s' "${compact}" |
      sed -n "s/.*\"description\":\"${escaped_description}\"[^]]*\"token\":\"\\([^\"]*\\)\".*/\\1/p" |
      head -n 1
  )"
  [[ -n "${token}" ]] || return 1
  printf '%s\n' "${token}"
}

lookup_existing_miniflux_api_key_token() {
  local response status body token
  response="$(
    curl -sS \
      -u "${MINIFLUX_ADMIN_USERNAME}:${MINIFLUX_ADMIN_PASSWORD}" \
      -w $'\n%{http_code}' \
      "${MINIFLUX_BASE_URL%/}/v1/api-keys"
  )"
  body="${response%$'\n'*}"
  status="${response##*$'\n'}"
  [[ "${status}" =~ ^2[0-9][0-9]$ ]] || return 1

  token="$(miniflux_api_key_token_from_list "${body}" "${MINIFLUX_API_KEY_DESCRIPTION}" || true)"
  [[ -n "${token}" ]] || return 1

  curl -fsS -H "X-Auth-Token: ${token}" "${MINIFLUX_BASE_URL%/}/v1/me" >/dev/null
  printf '%s\n' "${token}"
}

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

  local listed_token
  listed_token="$(lookup_existing_miniflux_api_key_token || true)"
  if [[ -n "${listed_token}" ]]; then
    printf '%s\n' "${listed_token}"
    return 0
  fi

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
    if [[ "${status}" == "400" ]]; then
      listed_token="$(lookup_existing_miniflux_api_key_token || true)"
      if [[ -n "${listed_token}" ]]; then
        printf '%s\n' "${listed_token}"
        return 0
      fi
    fi
    fail "创建 Miniflux API Key 失败: status=${status:-unknown} body=${body:-empty}"
  fi

  token="$(json_string_field "${body}" "token")"

  curl -fsS -H "X-Auth-Token: ${token}" "${MINIFLUX_BASE_URL%/}/v1/me" >/dev/null
  printf '%s\n' "${token}"
}
