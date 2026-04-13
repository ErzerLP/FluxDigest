#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOURCE_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

APP_ROOT="${APP_ROOT:-/opt/fluxdigest}"
APP_USER="${APP_USER:-}"
APP_GROUP="${APP_GROUP:-}"
ENV_FILE="${ENV_FILE:-/etc/fluxdigest/fluxdigest.env}"
SERVICE_DIR="${SERVICE_DIR:-/etc/systemd/system}"
SYSTEMD_TEMPLATE_DIR="${SOURCE_DIR}/deploy/systemd"
RELEASE_ID="${RELEASE_ID:-$(date +%Y%m%d%H%M%S)}"
RELEASE_DIR="${APP_ROOT}/releases/${RELEASE_ID}"
CURRENT_DIR="${APP_ROOT}/current"
BUILD_DIR="${SOURCE_DIR}/.build/systemd"
TMP_DIR="${BUILD_DIR}/tmp"
ENV_EXAMPLE="${SYSTEMD_TEMPLATE_DIR}/fluxdigest.env.example"
HEALTH_ENDPOINT="${HEALTH_ENDPOINT:-/healthz}"
HEALTH_RETRY="${HEALTH_RETRY:-30}"
HEALTH_INTERVAL="${HEALTH_INTERVAL:-2}"
RELEASE_RETENTION="${RELEASE_RETENTION:-5}"
SKIP_BUILD="${SKIP_BUILD:-0}"

usage() {
  cat <<'EOF'
Usage: deploy-systemd.sh [options]

Options:
  --app-root <path>      安装根目录，默认 /opt/fluxdigest
  --app-user <user>      systemd 运行用户，默认当前 sudo 用户或 fluxdigest
  --env-file <path>      共享 env 文件路径，默认 /etc/fluxdigest/fluxdigest.env
  --service-dir <path>   systemd unit 目录，默认 /etc/systemd/system
  --source-dir <path>    源码目录，默认脚本上两级目录
  --release-retention <n> 保留最近 n 个 release（0 表示关闭清理），默认 5
  --skip-build           跳过 go/npm 构建，直接安装已有产物
  -h, --help             显示帮助
EOF
}

default_app_user() {
  if [[ -n "${SUDO_USER:-}" && "${SUDO_USER}" != "root" ]]; then
    printf '%s\n' "${SUDO_USER}"
    return
  fi
  if [[ "$(id -u)" -ne 0 ]]; then
    id -un
    return
  fi
  printf '%s\n' "fluxdigest"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --app-root)
      APP_ROOT="$2"
      shift 2
      ;;
    --app-user)
      APP_USER="$2"
      shift 2
      ;;
    --env-file)
      ENV_FILE="$2"
      shift 2
      ;;
    --service-dir)
      SERVICE_DIR="$2"
      shift 2
      ;;
    --source-dir)
      SOURCE_DIR="$2"
      SYSTEMD_TEMPLATE_DIR="${SOURCE_DIR}/deploy/systemd"
      shift 2
      ;;
    --release-retention)
      RELEASE_RETENTION="$2"
      shift 2
      ;;
    --skip-build)
      SKIP_BUILD=1
      shift
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

if [[ ! "${RELEASE_RETENTION}" =~ ^[0-9]+$ ]]; then
  echo "--release-retention 必须是非负整数，当前值: ${RELEASE_RETENTION}" >&2
  exit 1
fi

if [[ -z "${APP_USER}" ]]; then
  APP_USER="$(default_app_user)"
fi

SUDO=""
if [[ "${EUID}" -ne 0 ]]; then
  if command -v sudo >/dev/null 2>&1; then
    SUDO="sudo"
  else
    echo "需要 root 或 sudo 权限来安装 systemd 与 env 文件" >&2
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
need_cmd install
need_cmd sed
need_cmd curl
need_cmd find
need_cmd sort
need_cmd readlink
need_cmd grep

if [[ "${SKIP_BUILD}" != "1" ]]; then
  need_cmd go
  need_cmd npm
fi

if [[ ! -f "${ENV_EXAMPLE}" ]]; then
  echo "未找到 env 示例文件: ${ENV_EXAMPLE}" >&2
  exit 1
fi

if [[ ! -d "${SOURCE_DIR}/cmd/rss-api" ]]; then
  echo "源码目录不正确: ${SOURCE_DIR}" >&2
  exit 1
fi

mkdir -p "${BUILD_DIR}" "${TMP_DIR}"

build_runtime_assets() {
  rm -rf "${BUILD_DIR}/bin" "${BUILD_DIR}/web-dist" "${TMP_DIR}"
  mkdir -p "${BUILD_DIR}/bin" "${BUILD_DIR}/web-dist" "${TMP_DIR}"

  echo "==> 安装前端依赖并构建 WebUI"
  npm --prefix "${SOURCE_DIR}/web" ci
  npm --prefix "${SOURCE_DIR}/web" run build

  echo "==> 构建 Go 二进制"
  CGO_ENABLED=0 go build -trimpath -o "${BUILD_DIR}/bin/rss-api" "${SOURCE_DIR}/cmd/rss-api"
  CGO_ENABLED=0 go build -trimpath -o "${BUILD_DIR}/bin/rss-worker" "${SOURCE_DIR}/cmd/rss-worker"
  CGO_ENABLED=0 go build -trimpath -o "${BUILD_DIR}/bin/rss-scheduler" "${SOURCE_DIR}/cmd/rss-scheduler"

  cp -R "${SOURCE_DIR}/web/dist/." "${BUILD_DIR}/web-dist/"
}

ensure_app_user() {
  if id -u "${APP_USER}" >/dev/null 2>&1; then
    return
  fi

  echo "==> 创建运行用户 ${APP_USER}"
  ${SUDO} useradd --system --user-group --create-home --shell /usr/sbin/nologin "${APP_USER}"
}

ensure_env_file() {
  local env_dir
  env_dir="$(dirname "${ENV_FILE}")"
  ${SUDO} install -d -m 0755 "${env_dir}"

  if [[ -f "${ENV_FILE}" ]]; then
    echo "==> 保留已有 env 文件 ${ENV_FILE}"
    return
  fi

  echo "==> 初始化 env 文件 ${ENV_FILE}"
  local tmp_env="${TMP_DIR}/fluxdigest.env"
  cp "${ENV_EXAMPLE}" "${tmp_env}"

  local env_keys=(
    GIN_MODE
    http_proxy
    https_proxy
    HTTP_PROXY
    HTTPS_PROXY
    APP_HTTP_PORT
    APP_DATABASE_DSN
    APP_REDIS_ADDR
    APP_JOB_API_KEY
    APP_JOB_QUEUE
    APP_WORKER_CONCURRENCY
    APP_MINIFLUX_BASE_URL
    APP_MINIFLUX_AUTH_TOKEN
    APP_LLM_BASE_URL
    APP_LLM_API_KEY
    APP_LLM_MODEL
    APP_LLM_FALLBACK_MODELS
    APP_LLM_TIMEOUT_MS
    APP_PUBLISH_CHANNEL
    APP_PUBLISH_HALO_BASE_URL
    APP_PUBLISH_HALO_TOKEN
    APP_PUBLISH_OUTPUT_DIR
  )

  for key in "${env_keys[@]}"; do
    local current_value="${!key-}"
    if [[ -z "${current_value}" ]]; then
      continue
    fi
    local escaped
    escaped="$(printf '%s' "${current_value}" | sed -e 's/[\/&]/\\&/g')"
    sed -i -E "s|^${key}=.*$|${key}=${escaped}|" "${tmp_env}"
  done

  ${SUDO} install -m 0640 "${tmp_env}" "${ENV_FILE}"
  ${SUDO} chown root:"${APP_GROUP}" "${ENV_FILE}"
}

load_env_file() {
  local runtime_env
  runtime_env="$(mktemp)"
  trap 'rm -f "${runtime_env}"; trap - RETURN' RETURN

  ${SUDO} cat "${ENV_FILE}" > "${runtime_env}"
  set -a
  # shellcheck disable=SC1090
  source "${runtime_env}"
  set +a
}

ensure_prebuilt_assets() {
  local required_files=(
    "${BUILD_DIR}/bin/rss-api"
    "${BUILD_DIR}/bin/rss-worker"
    "${BUILD_DIR}/bin/rss-scheduler"
    "${BUILD_DIR}/web-dist/index.html"
  )
  for path in "${required_files[@]}"; do
    if [[ ! -f "${path}" ]]; then
      echo "缺少预构建产物: ${path}" >&2
      echo "请先执行一次完整构建，或取消 --skip-build" >&2
      exit 1
    fi
  done
}

install_release() {
  echo "==> 安装 release 到 ${RELEASE_DIR}"
  ${SUDO} install -d -m 0755 "${APP_ROOT}" "${APP_ROOT}/releases" "${RELEASE_DIR}" "${RELEASE_DIR}/bin" "${RELEASE_DIR}/migrations" "${RELEASE_DIR}/web/dist"

  ${SUDO} install -m 0755 "${BUILD_DIR}/bin/rss-api" "${RELEASE_DIR}/bin/rss-api"
  ${SUDO} install -m 0755 "${BUILD_DIR}/bin/rss-worker" "${RELEASE_DIR}/bin/rss-worker"
  ${SUDO} install -m 0755 "${BUILD_DIR}/bin/rss-scheduler" "${RELEASE_DIR}/bin/rss-scheduler"
  ${SUDO} cp -R "${BUILD_DIR}/web-dist/." "${RELEASE_DIR}/web/dist/"

  ${SUDO} cp -R "${SOURCE_DIR}/migrations/." "${RELEASE_DIR}/migrations/"
  ${SUDO} chown -R "${APP_USER}:${APP_GROUP}" "${RELEASE_DIR}"
}

switch_current_release() {
  echo "==> 切换 current -> ${RELEASE_DIR}"
  ${SUDO} ln -sfn "${RELEASE_DIR}" "${CURRENT_DIR}"
}

ensure_runtime_directories() {
  local output_dir="${APP_PUBLISH_OUTPUT_DIR:-}"
  if [[ -n "${output_dir}" ]]; then
    ${SUDO} install -d -m 0755 "${output_dir}"
    ${SUDO} chown -R "${APP_USER}:${APP_GROUP}" "${output_dir}"
  fi
}

render_unit() {
  local template_name="$1"
  local target_name="$2"
  local template_path="${SYSTEMD_TEMPLATE_DIR}/${template_name}"
  local tmp_unit="${TMP_DIR}/${target_name}"
  local escaped_app_dir escaped_env_file escaped_app_user escaped_app_group

  if [[ ! -f "${template_path}" ]]; then
    echo "未找到模板: ${template_path}" >&2
    exit 1
  fi

  escaped_app_dir="$(printf '%s' "${CURRENT_DIR}" | sed -e 's/[\/&]/\\&/g')"
  escaped_env_file="$(printf '%s' "${ENV_FILE}" | sed -e 's/[\/&]/\\&/g')"
  escaped_app_user="$(printf '%s' "${APP_USER}" | sed -e 's/[\/&]/\\&/g')"
  escaped_app_group="$(printf '%s' "${APP_GROUP}" | sed -e 's/[\/&]/\\&/g')"

  sed \
    -e "s|__APP_DIR__|${escaped_app_dir}|g" \
    -e "s|__ENV_FILE__|${escaped_env_file}|g" \
    -e "s|__APP_USER__|${escaped_app_user}|g" \
    -e "s|__APP_GROUP__|${escaped_app_group}|g" \
    "${template_path}" > "${tmp_unit}"

  ${SUDO} install -m 0644 "${tmp_unit}" "${SERVICE_DIR}/${target_name}"
}

reload_and_restart() {
  echo "==> 重新加载并重启 systemd 服务"
  ${SUDO} systemctl daemon-reload || return 1
  ${SUDO} systemctl enable --now fluxdigest-api.service fluxdigest-worker.service fluxdigest-scheduler.service || return 1
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

recover_from_failed_deploy() {
  local failed_stage="$1"
  local previous_current="$2"

  echo "!! 部署失败阶段: ${failed_stage}" >&2

  if [[ -n "${previous_current}" ]] && ${SUDO} test -d "${previous_current}"; then
    echo "!! 尝试恢复 current 到部署前 release: ${previous_current}" >&2
    ${SUDO} ln -sfn "${previous_current}" "${CURRENT_DIR}" || true
    echo "!! 尝试恢复后重载并重启服务" >&2
    ${SUDO} systemctl daemon-reload || true
    ${SUDO} systemctl restart fluxdigest-api.service fluxdigest-worker.service fluxdigest-scheduler.service || true
  else
    echo "!! 未检测到可恢复的旧 current（可能是首次部署），无法执行回滚恢复" >&2
  fi

  echo "!! 当前服务状态（失败路径）" >&2
  print_service_status
}

cleanup_old_releases() {
  if [[ "${RELEASE_RETENTION}" -eq 0 ]]; then
    echo "==> 已关闭旧 release 清理"
    return
  fi

  local releases_dir="${APP_ROOT}/releases"
  if ! ${SUDO} test -d "${releases_dir}"; then
    return
  fi

  local current_target=""
  if ${SUDO} test -L "${CURRENT_DIR}"; then
    current_target="$(${SUDO} readlink -f "${CURRENT_DIR}" 2>/dev/null || true)"
  fi

  local -a release_ids=()
  mapfile -t release_ids < <(
    ${SUDO} find "${releases_dir}" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' \
      | grep -E '^[0-9]{14}$' \
      | sort -r || true
  )

  local total="${#release_ids[@]}"
  if (( total <= RELEASE_RETENTION )); then
    echo "==> release 保留策略：当前 ${total} 个，无需清理"
    return
  fi

  echo "==> 清理旧 release（保留最近 ${RELEASE_RETENTION} 个）"
  local idx
  for ((idx = RELEASE_RETENTION; idx < total; idx++)); do
    local release_id="${release_ids[idx]}"
    local target_dir="${releases_dir}/${release_id}"
    local resolved_target
    resolved_target="$(${SUDO} readlink -f "${target_dir}" 2>/dev/null || true)"

    if [[ -z "${resolved_target}" ]]; then
      echo "  - 跳过无法解析路径: ${target_dir}"
      continue
    fi
    if [[ -n "${current_target}" && "${resolved_target}" == "${current_target}" ]]; then
      echo "  - 跳过 current 指向 release: ${release_id}"
      continue
    fi
    if [[ "${resolved_target}" != "${releases_dir}/"* ]]; then
      echo "  - 跳过可疑路径: ${resolved_target}"
      continue
    fi

    ${SUDO} rm -rf -- "${target_dir}"
    echo "  - 已删除旧 release: ${release_id}"
  done
}

main() {
  local previous_current=""

  ensure_app_user
  if [[ -z "${APP_GROUP}" ]]; then
    APP_GROUP="$(id -gn "${APP_USER}")"
  fi

  if [[ "${SKIP_BUILD}" != "1" ]]; then
    build_runtime_assets
  else
    ensure_prebuilt_assets
  fi

  ensure_env_file
  load_env_file
  ensure_runtime_directories
  if ${SUDO} test -L "${CURRENT_DIR}"; then
    previous_current="$(${SUDO} readlink -f "${CURRENT_DIR}" 2>/dev/null || true)"
  fi
  install_release
  render_unit "fluxdigest-api.service.tpl" "fluxdigest-api.service"
  render_unit "fluxdigest-worker.service.tpl" "fluxdigest-worker.service"
  render_unit "fluxdigest-scheduler.service.tpl" "fluxdigest-scheduler.service"
  switch_current_release
  if ! reload_and_restart; then
    recover_from_failed_deploy "restart" "${previous_current}"
    exit 1
  fi
  if ! health_check; then
    recover_from_failed_deploy "health-check" "${previous_current}"
    exit 1
  fi
  cleanup_old_releases

  echo "==> 部署完成"
  echo "current release: ${RELEASE_DIR}"
}

main "$@"
