#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

halo_pat_payload() {
  cat <<EOF
{
  "apiVersion": "security.halo.run/v1alpha1",
  "kind": "PersonalAccessToken",
  "metadata": {
    "generateName": "fluxdigest-pat-"
  },
  "spec": {
    "name": "${HALO_PAT_NAME}",
    "description": "FluxDigest publisher token",
    "username": "${HALO_ADMIN_USERNAME}",
    "roles": ["superadmin"],
    "scopes": ["*"],
    "tokenId": ""
  }
}
EOF
}

create_halo_pat() {
  local payload="${1:?}"
  local base="${HALO_BASE_URL%/}"
  local endpoints=(
    "${base}/apis/uc.api.security.halo.run/v1alpha1/personalaccesstokens"
    "${base}/apis/api.uc.halo.run/v1alpha1/personalAccessTokens"
  )

  local endpoint response status body last_status="" last_body=""
  for endpoint in "${endpoints[@]}"; do
    response="$(
      curl -sS \
        -u "${HALO_ADMIN_USERNAME}:${HALO_ADMIN_PASSWORD}" \
        -H 'Content-Type: application/json' \
        -X POST \
        -d "${payload}" \
        -w $'\n%{http_code}' \
        "${endpoint}" || true
    )"
    body="${response%$'\n'*}"
    status="${response##*$'\n'}"
    if [[ "${status}" =~ ^2[0-9][0-9]$ ]]; then
      printf '%s\n' "${body}"
      return 0
    fi
    last_status="${status}"
    last_body="${body}"
  done

  fail "创建 Halo PAT 失败: status=${last_status:-unknown} body=${last_body:-empty}"
}

bootstrap_halo() {
  [[ -n "${HALO_BASE_URL:-}" ]] || fail "HALO_BASE_URL is required"

  local existing_token="${APP_PUBLISH_HALO_TOKEN:-}"
  if [[ -n "${existing_token}" ]] && curl -fsS -H "Authorization: Bearer ${existing_token}" "${HALO_BASE_URL%/}/apis/api.console.halo.run/v1alpha1/posts?page=1&size=1" >/dev/null; then
    printf '%s\n' "${existing_token}"
    return 0
  fi

  [[ -n "${HALO_ADMIN_USERNAME:-}" ]] || fail "HALO_ADMIN_USERNAME is required"
  [[ -n "${HALO_ADMIN_PASSWORD:-}" ]] || fail "HALO_ADMIN_PASSWORD is required"
  [[ -n "${HALO_PAT_NAME:-}" ]] || fail "HALO_PAT_NAME is required"

  local response token
  response="$(create_halo_pat "$(halo_pat_payload)")"
  token="$(json_string_field "${response}" "tokenId")"

  curl -fsS -H "Authorization: Bearer ${token}" "${HALO_BASE_URL%/}/apis/api.console.halo.run/v1alpha1/posts?page=1&size=1" >/dev/null
  printf '%s\n' "${token}"
}
