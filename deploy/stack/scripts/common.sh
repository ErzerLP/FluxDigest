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
  local profile="${1:-${STACK_PROFILE:-fluxdigest-only}}"
  local service="${2:?}"
  local extra_list="${STACK_PROFILE_SERVICES:-}"
  local candidates=()

  while IFS= read -r entry; do
    candidates+=("$entry")
  done < <(collect_service_entries "$extra_list")

  case "$profile" in
    fluxdigest-only)
      candidates+=(postgres redis fluxdigest-api)
      ;;
    fluxdigest-miniflux)
      candidates+=(postgres redis fluxdigest-api miniflux)
      ;;
    fluxdigest-halo)
      candidates+=(postgres redis fluxdigest-api halo)
      ;;
    fluxdigest-full)
      candidates+=(postgres redis fluxdigest-api miniflux halo)
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
  local python_cmd=""
  if command -v python >/dev/null 2>&1; then
    python_cmd=python
  elif command -v python3 >/dev/null 2>&1; then
    python_cmd=python3
  else
    fail "无法生成随机 token：缺少 python3/python"
  fi

  "$python_cmd" <<'PY' "$length"
import secrets
import sys
size = int(sys.argv[1])
print(secrets.token_urlsafe(size))
PY
}
