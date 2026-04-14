#!/usr/bin/env bash

set -euo pipefail

log() {
  printf '%s %s\n' "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" "$*"
}

log_info() {
  log "[INFO]" "$@"
}

log_warn() {
  log "[WARN]" "$@"
}

log_error() {
  log "[ERROR]" "$@" >&2
}

fail() {
  log_error "$@"
  exit 1
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    fail "缺少必要命令: $1"
  fi
}

trim_spaces() {
  local value="$1"
  [[ -z "$value" ]] && return
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "$value"
}

collect_service_entries() {
  local list="$1"
  [[ -z "$list" ]] && return
  IFS=',' read -ra items <<< "$list"
  local item
  for item in "${items[@]}"; do
    local trimmed
    trimmed="$(trim_spaces "$item")"
    [[ -z "$trimmed" ]] && continue
    if [[ "$trimmed" =~ [[:space:]] ]]; then
      IFS=$' \t' read -ra tokens <<< "$trimmed"
      local token
      for token in "${tokens[@]}"; do
        local normalized
        normalized="$(trim_spaces "$token")"
        [[ -n "$normalized" ]] && printf '%s\n' "$normalized"
      done
    else
      printf '%s\n' "$trimmed"
    fi
  done
}

profile_has_service() {
  local profile="${1:-${STACK_PROFILE:-full}}"
  local service="${2:?}"
  local candidates=()

  case "$profile" in
    full)
      candidates+=(postgres redis fluxdigest-api fluxdigest-worker fluxdigest-scheduler miniflux halo)
      ;;
    fluxdigest-only)
      candidates+=(postgres redis fluxdigest-api fluxdigest-worker fluxdigest-scheduler)
      ;;
    fluxdigest-miniflux)
      candidates+=(postgres redis fluxdigest-api fluxdigest-worker fluxdigest-scheduler miniflux)
      ;;
    fluxdigest-halo)
      candidates+=(postgres redis fluxdigest-api fluxdigest-worker fluxdigest-scheduler halo)
      ;;
    fluxdigest-full)
      candidates+=(postgres redis fluxdigest-api fluxdigest-worker fluxdigest-scheduler miniflux halo)
      ;;
    *)
      while IFS= read -r entry; do
        candidates+=("$entry")
      done < <(collect_service_entries "$profile")
      ;;
  esac

  local candidate
  for candidate in "${candidates[@]}"; do
    [[ -z "$candidate" ]] && continue
    [[ "$candidate" == "$service" ]] && return 0
  done

  return 1
}

random_token() {
  local length="${1:-24}"
  local hex_len
  hex_len=$((length * 2))
  openssl rand -hex "${length}" | awk -v max="${hex_len}" '{print substr($0,1,max)}'
}

json_string_field() {
  local payload="${1:?}"
  local field="${2:?}"
  local value
  value="$(
    printf '%s' "${payload}" |
      tr -d '\r\n' |
      sed -n "s/.*\"${field}\"[[:space:]]*:[[:space:]]*\"\\([^\"]*\\)\".*/\\1/p" |
      head -n 1
  )"

  [[ -n "${value}" ]] || fail "无法从 JSON 中提取字段: ${field}"
  printf '%s\n' "${value}"
}
