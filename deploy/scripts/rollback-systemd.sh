#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

APP_ROOT="${APP_ROOT:-/opt/fluxdigest}"
ENV_FILE="${ENV_FILE:-/etc/fluxdigest/fluxdigest.env}"
CURRENT_DIR="${APP_ROOT}/current"
HEALTH_ENDPOINT="${HEALTH_ENDPOINT:-/healthz}"
HEALTH_RETRY="${HEALTH_RETRY:-30}"
HEALTH_INTERVAL="${HEALTH_INTERVAL:-2}"
TARGET_RELEASE_ID=""

usage() {
  cat <<'EOF'
Usage: rollback-systemd.sh [options]

Options:
  --app-root <path>      安装根目录，默认 /opt/fluxdigest
  --env-file <path>      共享 env 文件路径，默认 /etc/fluxdigest/fluxdigest.env
  --release-id <id>      指定回滚目标 release ID；不传则自动回滚到上一个 release
  -h, --help             显示帮助
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --app-root)
      APP_ROOT="$2"
      CURRENT_DIR="${APP_ROOT}/current"
      shift 2
      ;;
    --env-file)
      ENV_FILE="$2"
      shift 2
      ;;
    --release-id)
      TARGET_RELEASE_ID="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

SUDO=""
if [[ "${EUID}" -ne 0 ]]; then
  if command -v sudo >/dev/null 2>&1; then
    SUDO="sudo"
  else
    echo "需要 root 或 sudo 权限执行回滚" >&2
    exit 1
  fi
fi

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "缺少依赖命令: $1" >&2
    exit 1
  fi
}

need_cmd systemctl
need_cmd curl
need_cmd find
need_cmd sort
need_cmd readlink
need_cmd grep

load_env_file() {
  if ! ${SUDO} test -f "${ENV_FILE}"; then
    return
  fi

  local runtime_env
  runtime_env="$(mktemp)"
  ${SUDO} cat "${ENV_FILE}" > "${runtime_env}"
  set -a
  # shellcheck disable=SC1090
  source "${runtime_env}"
  set +a
  rm -f "${runtime_env}"
}

resolve_current_release() {
  if ! ${SUDO} test -L "${CURRENT_DIR}"; then
    echo "current 链接不存在: ${CURRENT_DIR}" >&2
    exit 1
  fi

  local current_target
  current_target="$(${SUDO} readlink -f "${CURRENT_DIR}" 2>/dev/null || true)"
  if [[ -z "${current_target}" ]] || ! ${SUDO} test -d "${current_target}"; then
    echo "current 链接目标无效: ${CURRENT_DIR}" >&2
    exit 1
  fi

  printf '%s\n' "${current_target}"
}

is_valid_release_dir() {
  local path="$1"
  ${SUDO} test -d "${path}" \
    && ${SUDO} test -f "${path}/bin/rss-api" \
    && ${SUDO} test -f "${path}/bin/rss-worker" \
    && ${SUDO} test -f "${path}/bin/rss-scheduler"
}

find_target_release() {
  local releases_dir="${APP_ROOT}/releases"
  local current_target="$1"

  if ! ${SUDO} test -d "${releases_dir}"; then
    echo "release 目录不存在: ${releases_dir}" >&2
    exit 1
  fi

  if [[ -n "${TARGET_RELEASE_ID}" ]]; then
    local specified="${releases_dir}/${TARGET_RELEASE_ID}"
    if ! is_valid_release_dir "${specified}"; then
      echo "指定 release 不存在或结构不完整: ${specified}" >&2
      exit 1
    fi
    local resolved_specified
    resolved_specified="$(${SUDO} readlink -f "${specified}" 2>/dev/null || true)"
    if [[ "${resolved_specified}" == "${current_target}" ]]; then
      echo "指定 release 与 current 相同，拒绝回滚: ${TARGET_RELEASE_ID}" >&2
      exit 1
    fi
    printf '%s\n' "${specified}"
    return
  fi

  local -a candidates=()
  mapfile -t candidates < <(
    ${SUDO} find "${releases_dir}" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' \
      | grep -E '^[0-9]{14}$' \
      | sort -r || true
  )

  local -a ordered_valid=()
  local name path resolved
  for name in "${candidates[@]}"; do
    path="${releases_dir}/${name}"
    if ! is_valid_release_dir "${path}"; then
      continue
    fi
    resolved="$(${SUDO} readlink -f "${path}" 2>/dev/null || true)"
    if [[ -z "${resolved}" ]]; then
      continue
    fi
    ordered_valid+=("${path}")
  done

  if [[ "${#ordered_valid[@]}" -eq 0 ]]; then
    echo "未找到可用的 release（目录: ${releases_dir}）" >&2
    exit 1
  fi

  local current_index=-1
  local i candidate_path candidate_resolved
  for ((i = 0; i < ${#ordered_valid[@]}; i++)); do
    candidate_path="${ordered_valid[i]}"
    candidate_resolved="$(${SUDO} readlink -f "${candidate_path}" 2>/dev/null || true)"
    if [[ "${candidate_resolved}" == "${current_target}" ]]; then
      current_index="${i}"
      break
    fi
  done

  if [[ "${current_index}" -lt 0 ]]; then
    echo "current 不在有效 release 列表中，无法自动回滚（current: ${current_target}）" >&2
    exit 1
  fi

  local prev_index=$((current_index + 1))
  if [[ "${prev_index}" -ge "${#ordered_valid[@]}" ]]; then
    echo "当前 release 已是最旧版本，无法回滚到更旧版本（current: $(basename "${current_target}"))" >&2
    exit 1
  fi

  printf '%s\n' "${ordered_valid[prev_index]}"
  return
}

reload_and_restart() {
  echo "==> 重新加载并重启 systemd 服务"
  ${SUDO} systemctl daemon-reload || return 1
  ${SUDO} systemctl restart fluxdigest-api.service fluxdigest-worker.service fluxdigest-scheduler.service || return 1
}

print_service_status() {
  ${SUDO} systemctl status --no-pager fluxdigest-api.service fluxdigest-worker.service fluxdigest-scheduler.service >&2 || true
}

health_check() {
  local port="${APP_HTTP_PORT:-8080}"
  local url="http://127.0.0.1:${port}${HEALTH_ENDPOINT}"

  echo "==> 验活 ${url}"
  for ((i = 1; i <= HEALTH_RETRY; i++)); do
    if curl --silent --show-error --fail "${url}" >/dev/null; then
      echo "health check ok"
      return 0
    fi
    sleep "${HEALTH_INTERVAL}"
  done

  echo "health check failed: ${url}" >&2
  print_service_status
  return 1
}

recover_from_failed_rollback() {
  local previous_release="$1"
  local attempted_release="$2"
  local failed_stage="$3"

  echo "!! 回滚失败阶段: ${failed_stage}" >&2
  echo "!! 尝试恢复 current 到原 release: ${previous_release}" >&2
  ${SUDO} ln -sfn "${previous_release}" "${CURRENT_DIR}" || true

  echo "!! 尝试恢复后重启服务" >&2
  ${SUDO} systemctl daemon-reload || true
  ${SUDO} systemctl restart fluxdigest-api.service fluxdigest-worker.service fluxdigest-scheduler.service || true

  echo "!! 当前服务状态（失败路径）" >&2
  print_service_status
  echo "!! 已从失败目标回滚恢复: ${attempted_release} -> ${previous_release}" >&2
}

main() {
  load_env_file

  local current_release target_release resolved_target releases_dir
  releases_dir="${APP_ROOT}/releases"
  current_release="$(resolve_current_release)"
  target_release="$(find_target_release "${current_release}")"
  resolved_target="$(${SUDO} readlink -f "${target_release}" 2>/dev/null || true)"

  if [[ -z "${resolved_target}" || "${resolved_target}" != "${releases_dir}/"* ]]; then
    echo "目标 release 路径异常，已中止: ${target_release}" >&2
    exit 1
  fi

  echo "==> 回滚 current -> ${resolved_target}"
  ${SUDO} ln -sfn "${resolved_target}" "${CURRENT_DIR}"
  if ! reload_and_restart; then
    recover_from_failed_rollback "${current_release}" "${resolved_target}" "restart"
    exit 1
  fi
  if ! health_check; then
    recover_from_failed_rollback "${current_release}" "${resolved_target}" "health-check"
    exit 1
  fi

  echo "==> 回滚完成"
  echo "current release: ${resolved_target}"
}

main "$@"
