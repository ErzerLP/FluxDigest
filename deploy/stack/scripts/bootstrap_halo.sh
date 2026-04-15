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

halo_base64_encode() {
  local raw="${1:-}"
  local python_cmd

  if python_cmd="$(preferred_python_cmd)"; then
    HALO_BASE64_RAW="${raw}" "${python_cmd}" - <<'PY'
import base64
import os

print(base64.b64encode(os.environ.get("HALO_BASE64_RAW", "").encode()).decode())
PY
    return 0
  fi

  if command -v base64 >/dev/null 2>&1; then
    printf '%s' "${raw}" | base64 | tr -d '\r\n'
    return 0
  fi

  return 1
}

halo_auth_header_value() {
  local token="${1:-}"
  if [[ -z "${token}" ]]; then
    return 1
  fi

  if [[ "${token,,}" == basic:* ]]; then
    printf 'Basic %s\n' "${token#*:}"
    return 0
  fi

  printf 'Bearer %s\n' "${token}"
}

halo_validate_publish_token() {
  local token="${1:-}"
  [[ -n "${token}" ]] || return 1

  local auth_header
  auth_header="$(halo_auth_header_value "${token}")" || return 1

  curl -fsS \
    -H "Authorization: ${auth_header}" \
    "${HALO_BASE_URL%/}/apis/api.console.halo.run/v1alpha1/posts?page=1&size=1" >/dev/null
}

halo_basic_token() {
  [[ -n "${HALO_ADMIN_USERNAME:-}" ]] || fail "HALO_ADMIN_USERNAME is required"
  [[ -n "${HALO_ADMIN_PASSWORD:-}" ]] || fail "HALO_ADMIN_PASSWORD is required"

  local encoded
  encoded="$(halo_base64_encode "${HALO_ADMIN_USERNAME}:${HALO_ADMIN_PASSWORD}")" || fail "无法生成 Halo Basic token"
  printf 'basic:%s\n' "${encoded}"
}

halo_access_token_from_response() {
  local payload="${1:?}"
  local python_cmd
  if python_cmd="$(preferred_python_cmd)"; then
    HALO_PAT_PAYLOAD="${payload}" "${python_cmd}" - <<'PY'
import json
import os
import sys

payload = os.environ.get("HALO_PAT_PAYLOAD", "")

try:
    data = json.loads(payload)
except Exception:
    sys.exit(1)

token = (
    data.get("metadata", {})
    .get("annotations", {})
    .get("security.halo.run/access-token", "")
)
if token:
    print(token)
    sys.exit(0)

sys.exit(1)
PY
    return $?
  fi

  local token
  token="$(
    printf '%s' "${payload}" |
      tr -d '\r\n' |
      sed -n 's/.*"security\.halo\.run\/access-token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' |
      head -n 1
  )"
  [[ -n "${token}" ]] || return 1
  printf '%s\n' "${token}"
}

ensure_halo_initialized() {
  local base="${HALO_BASE_URL%/}"
  local tmp_dir cookies_file setup_html csrf response status body site_title
  site_title="${HALO_SITE_TITLE:-FluxDigest}"
  tmp_dir="$(mktemp -d)"
  cookies_file="${tmp_dir}/cookies.txt"
  setup_html="${tmp_dir}/setup.html"
  local attempt max_attempts sleep_seconds
  max_attempts="${HALO_SETUP_READY_ATTEMPTS:-30}"
  sleep_seconds="${HALO_SETUP_READY_SLEEP_SECONDS:-3}"

  for attempt in $(seq 1 "${max_attempts}"); do
    response="$(
      curl -sS \
        -c "${cookies_file}" \
        -o "${setup_html}" \
        -w '%{http_code}' \
        "${base}/system/setup" || true
    )"
    status="${response}"

    case "${status}" in
      302|303|307|308)
        rm -rf "${tmp_dir}"
        return 0
        ;;
      401|404)
        rm -rf "${tmp_dir}"
        return 0
        ;;
      200)
        if ! grep -q 'action="/system/setup"' "${setup_html}"; then
          rm -rf "${tmp_dir}"
          return 0
        fi
        [[ -n "${HALO_ADMIN_EMAIL:-}" ]] || fail "HALO_ADMIN_EMAIL is required"
        [[ -n "${HALO_EXTERNAL_URL:-}" ]] || fail "HALO_EXTERNAL_URL is required"
        break
        ;;
      500|502|503|000)
        if [[ "${attempt}" -lt "${max_attempts}" ]]; then
          log_warn "Halo setup 页面尚未就绪 (${attempt}/${max_attempts})：status=${status}" >&2
          sleep "${sleep_seconds}"
          continue
        fi
        fail "获取 Halo setup 页面失败: status=${status}"
        ;;
      *)
        fail "获取 Halo setup 页面失败: status=${status}"
        ;;
    esac
  done

  csrf="$(
    sed -n 's/.*name="_csrf" value="\([^"]*\)".*/\1/p' "${setup_html}" |
      head -n 1
  )"
  [[ -n "${csrf}" ]] || fail "无法从 Halo setup 页面提取 _csrf"

  response="$(
    curl -sS \
      -b "${cookies_file}" \
      -c "${cookies_file}" \
      -H 'Content-Type: application/x-www-form-urlencoded' \
      -X POST \
      -w $'\n%{http_code}' \
      "${base}/system/setup" \
      --data-urlencode "_csrf=${csrf}" \
      --data-urlencode "language=zh-CN" \
      --data-urlencode "externalUrl=${HALO_EXTERNAL_URL}" \
      --data-urlencode "siteTitle=${site_title}" \
      --data-urlencode "username=${HALO_ADMIN_USERNAME}" \
      --data-urlencode "email=${HALO_ADMIN_EMAIL}" \
      --data-urlencode "password=${HALO_ADMIN_PASSWORD}"
  )"
  body="${response%$'\n'*}"
  status="${response##*$'\n'}"
  if [[ "${status}" != "204" ]]; then
    fail "初始化 Halo 失败: status=${status:-unknown} body=${body:-empty}"
  fi

  rm -rf "${tmp_dir}"
}

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
  if [[ -n "${existing_token}" ]] && halo_validate_publish_token "${existing_token}"; then
    printf '%s\n' "${existing_token}"
    return 0
  fi

  [[ -n "${HALO_ADMIN_USERNAME:-}" ]] || fail "HALO_ADMIN_USERNAME is required"
  [[ -n "${HALO_ADMIN_PASSWORD:-}" ]] || fail "HALO_ADMIN_PASSWORD is required"
  [[ -n "${HALO_PAT_NAME:-}" ]] || fail "HALO_PAT_NAME is required"

  ensure_halo_initialized

  local response token
  response="$(create_halo_pat "$(halo_pat_payload)")"
  token="$(halo_access_token_from_response "${response}" || true)"

  if [[ -n "${token}" ]] && halo_validate_publish_token "${token}"; then
    printf '%s\n' "${token}"
    return 0
  fi

  if [[ -n "${token}" ]]; then
    log_warn "Halo PAT 已创建但无法访问 Console API，自动回退到 Basic 鉴权" >&2
  else
    log_warn "Halo PAT 响应未返回 access token，自动回退到 Basic 鉴权" >&2
  fi

  token="$(halo_basic_token)"
  halo_validate_publish_token "${token}" || fail "Halo Basic 鉴权验证失败"
  printf '%s\n' "${token}"
}
