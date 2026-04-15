#!/usr/bin/env bash

set -euo pipefail

INSTALL_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STACK_SOURCE_ROOT="$(cd "${INSTALL_SCRIPT_DIR}/../.." && pwd)"
STACK_PROFILE="${STACK_PROFILE:-full}"
STACK_DIR="${STACK_DIR:-/opt/fluxdigest-stack}"
STACK_ACTION="${STACK_ACTION:-install}"
STACK_RELEASE_ID_OVERRIDE="${STACK_RELEASE_ID_OVERRIDE:-}"
STACK_RELEASE_TARGET_ID="${STACK_RELEASE_TARGET_ID:-}"
STACK_PUBLIC_HOST="${STACK_PUBLIC_HOST:-}"
STACK_RELEASE_ID="${STACK_RELEASE_ID:-}"
STACK_RELEASES_DIR="${STACK_RELEASES_DIR:-}"
STACK_PROFILE_EXPLICIT=0
STACK_PUBLIC_HOST_EXPLICIT=0

source "${INSTALL_SCRIPT_DIR}/scripts/common.sh"
source "${INSTALL_SCRIPT_DIR}/scripts/render.sh"
source "${INSTALL_SCRIPT_DIR}/scripts/healthcheck.sh"

if [[ -f "${INSTALL_SCRIPT_DIR}/scripts/bootstrap_miniflux.sh" ]]; then
  # shellcheck disable=SC1091
  source "${INSTALL_SCRIPT_DIR}/scripts/bootstrap_miniflux.sh"
fi

if [[ -f "${INSTALL_SCRIPT_DIR}/scripts/bootstrap_halo.sh" ]]; then
  # shellcheck disable=SC1091
  source "${INSTALL_SCRIPT_DIR}/scripts/bootstrap_halo.sh"
fi

STACK_FORCE=0
STACK_SHOW_HELP=0

readonly FLUXDIGEST_HTTP_PORT=18088
readonly MINIFLUX_HTTP_PORT=28082
readonly HALO_HTTP_PORT=28090
readonly POSTGRES_HOST_PORT=35432
readonly REDIS_HOST_PORT=36379
readonly DEFAULT_DOCKER_SYSTEMD_DROPIN_DIR="/etc/systemd/system/docker.service.d"

usage() {
  cat <<'EOF'
Usage: install.sh [options]

Options:
  --action <name>    Action: install | upgrade | rollback | status
  --profile <name>   Stack profile: full | fluxdigest-miniflux | fluxdigest-halo | fluxdigest-only
  --stack-dir <dir>  Target stack directory (default: /opt/fluxdigest-stack)
  --release-id <id>  Release ID used by rollback
  --host <value>     Public host or IP for summary output
  --force            Allow overwriting existing generated files
  -h, --help         Show this help
EOF
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --action)
        [[ $# -ge 2 ]] || fail "--action 缺少参数"
        STACK_ACTION="$2"
        shift 2
        ;;
      --profile)
        [[ $# -ge 2 ]] || fail "--profile 缺少参数"
        STACK_PROFILE="$2"
        STACK_PROFILE_EXPLICIT=1
        shift 2
        ;;
      --stack-dir)
        [[ $# -ge 2 ]] || fail "--stack-dir 缺少参数"
        STACK_DIR="$2"
        shift 2
        ;;
      --release-id)
        [[ $# -ge 2 ]] || fail "--release-id 缺少参数"
        STACK_RELEASE_TARGET_ID="$2"
        shift 2
        ;;
      --host)
        [[ $# -ge 2 ]] || fail "--host 缺少参数"
        STACK_PUBLIC_HOST="$2"
        STACK_PUBLIC_HOST_EXPLICIT=1
        shift 2
        ;;
      --force)
        STACK_FORCE=1
        shift
        ;;
      -h|--help)
        STACK_SHOW_HELP=1
        shift
        ;;
      *)
        fail "未知参数: $1"
        ;;
    esac
  done

  case "${STACK_ACTION}" in
    install|upgrade|rollback|status) ;;
    *) fail "不支持的 action: ${STACK_ACTION}" ;;
  esac

  case "${STACK_PROFILE}" in
    full|fluxdigest-miniflux|fluxdigest-halo|fluxdigest-only) ;;
    *) fail "不支持的 profile: ${STACK_PROFILE}" ;;
  esac
}

ensure_linux() {
  local kernel
  kernel="$(uname -s)"
  [[ "${kernel}" == "Linux" ]] || fail "仅支持 Linux 安装，当前系统: ${kernel}"
}

ensure_required_commands() {
  local cmd
  for cmd in openssl sed awk curl sort find cmp; do
    require_cmd "${cmd}"
  done

  if [[ "${STACK_ACTION}" == "status" ]]; then
    return 0
  fi

  require_cmd docker
  if ! docker compose version >/dev/null 2>&1; then
    fail "缺少 docker compose（请确认 docker compose 子命令可用）"
  fi

  [[ -f "${STACK_SOURCE_ROOT}/deployments/docker/api.Dockerfile" ]] || fail "缺少 api.Dockerfile"
  [[ -f "${STACK_SOURCE_ROOT}/deployments/docker/worker.Dockerfile" ]] || fail "缺少 worker.Dockerfile"
  [[ -f "${STACK_SOURCE_ROOT}/deployments/docker/scheduler.Dockerfile" ]] || fail "缺少 scheduler.Dockerfile"
}

to_absolute_path() {
  local target="${1:?}"
  if [[ "${target}" = /* ]]; then
    printf '%s\n' "${target}"
    return 0
  fi

  local base_dir
  base_dir="$(dirname "${target}")"
  local leaf
  leaf="$(basename "${target}")"

  mkdir -p "${base_dir}"
  local abs_base
  abs_base="$(cd "${base_dir}" && pwd -P)"
  printf '%s/%s\n' "${abs_base}" "${leaf}"
}

merge_csv_values() {
  local merged=""
  local source raw item
  declare -A seen=()

  for source in "$@"; do
    [[ -n "${source}" ]] || continue
    IFS=',' read -ra raw <<< "${source}"
    for item in "${raw[@]}"; do
      item="$(trim_spaces "${item}")"
      [[ -n "${item}" ]] || continue
      if [[ -n "${seen[${item}]+x}" ]]; then
        continue
      fi
      seen["${item}"]=1
      if [[ -n "${merged}" ]]; then
        merged+=",${item}"
      else
        merged="${item}"
      fi
    done
  done

  printf '%s\n' "${merged}"
}

set_stack_paths() {
  export STACK_SOURCE_ROOT
  export STACK_PROFILE
  export STACK_PUBLIC_HOST

  export STACK_INITDB_DIR="${STACK_DIR}/initdb"
  export STACK_DATA_ROOT="${STACK_DIR}/data"
  export STACK_POSTGRES_DATA_DIR="${STACK_DATA_ROOT}/postgres"
  export STACK_REDIS_DATA_DIR="${STACK_DATA_ROOT}/redis"
  export STACK_MINIFLUX_DATA_DIR="${STACK_DATA_ROOT}/miniflux"
  export STACK_HALO_DATA_DIR="${STACK_DATA_ROOT}/halo"
  export STACK_FLUXDIGEST_DATA_DIR="${STACK_DATA_ROOT}/fluxdigest"
  export STACK_FLUXDIGEST_OUTPUT_DIR="${STACK_FLUXDIGEST_DATA_DIR}/output"
  export STACK_LOG_DIR="${STACK_DIR}/logs"
  export STACK_RELEASES_DIR="${STACK_DIR}/releases"
}

prepare_stack_dir() {
  STACK_DIR="$(to_absolute_path "${STACK_DIR}")"
  set_stack_paths

  local key_files=(
    "${STACK_DIR}/.env"
    "${STACK_DIR}/docker-compose.yml"
    "${STACK_DIR}/install-summary.txt"
  )

  local file
  case "${STACK_ACTION}" in
    install)
      if [[ "${STACK_FORCE}" -ne 1 ]]; then
        for file in "${key_files[@]}"; do
          if [[ -e "${file}" ]]; then
            fail "目标目录已存在关键文件，使用 --force 允许覆盖: ${file}"
          fi
        done
      fi
      ;;
    upgrade|rollback|status)
      for file in "${STACK_DIR}/.env" "${STACK_DIR}/docker-compose.yml"; do
        [[ -e "${file}" ]] || fail "未找到现有 stack 文件: ${file}"
      done
      ;;
  esac

  mkdir -p \
    "${STACK_INITDB_DIR}" \
    "${STACK_POSTGRES_DATA_DIR}" \
    "${STACK_REDIS_DATA_DIR}" \
    "${STACK_MINIFLUX_DATA_DIR}" \
    "${STACK_HALO_DATA_DIR}" \
    "${STACK_FLUXDIGEST_OUTPUT_DIR}" \
    "${STACK_LOG_DIR}" \
    "${STACK_RELEASES_DIR}"

  log_info "Using stack directory: ${STACK_DIR}"
}

setup_profile_service_blocks() {
  if profile_has_service "${STACK_PROFILE}" "miniflux"; then
    export STACK_MINIFLUX_SERVICE_BLOCK
    STACK_MINIFLUX_SERVICE_BLOCK=$(cat <<'EOF'
  miniflux:
    image: ${MINIFLUX_IMAGE}
    restart: unless-stopped
    env_file:
      - .env
    environment:
      DATABASE_URL: postgres://${MINIFLUX_DB_USER}:${MINIFLUX_DB_PASSWORD}@postgres:5432/${MINIFLUX_DB_NAME}?sslmode=disable
      RUN_MIGRATIONS: "1"
      CREATE_ADMIN: "1"
      ADMIN_USERNAME: ${MINIFLUX_ADMIN_USERNAME}
      ADMIN_PASSWORD: ${MINIFLUX_ADMIN_PASSWORD}
    ports:
      - "28082:8080"
    depends_on:
      postgres:
        condition: service_healthy
EOF
)
  else
    export STACK_MINIFLUX_SERVICE_BLOCK=""
  fi

  if profile_has_service "${STACK_PROFILE}" "halo"; then
    export STACK_HALO_SERVICE_BLOCK
    STACK_HALO_SERVICE_BLOCK=$(cat <<'EOF'
  halo:
    image: ${HALO_IMAGE}
    restart: unless-stopped
    env_file:
      - .env
    command:
      - --spring.r2dbc.url=r2dbc:pool:postgresql://postgres:5432/${HALO_DB_NAME}
      - --spring.r2dbc.username=${HALO_DB_USER}
      - --spring.r2dbc.password=${HALO_DB_PASSWORD}
      - --spring.sql.init.platform=postgresql
      - --server.port=8090
      - --halo.external-url=${HALO_EXTERNAL_URL}
      - --halo.security.initializer.superadminusername=${HALO_ADMIN_USERNAME}
      - --halo.security.initializer.superadminpassword=${HALO_ADMIN_PASSWORD}
      - --halo.security.basic-auth.disabled=false
    ports:
      - "28090:8090"
    volumes:
      - ${STACK_HALO_DATA_DIR}:/root/.halo2
    depends_on:
      postgres:
        condition: service_healthy
EOF
)
  else
    export STACK_HALO_SERVICE_BLOCK=""
  fi
}

generate_release_id() {
  if [[ -n "${STACK_RELEASE_ID_OVERRIDE}" ]]; then
    printf '%s\n' "${STACK_RELEASE_ID_OVERRIDE}"
    return 0
  fi
  date -u +'%Y%m%d%H%M%S'
}

set_release_image_tags() {
  STACK_RELEASE_ID="$(generate_release_id)"
  export FLUXDIGEST_RELEASE_ID="${STACK_RELEASE_ID}"
  export FLUXDIGEST_API_IMAGE="fluxdigest/api:${STACK_RELEASE_ID}"
  export FLUXDIGEST_WORKER_IMAGE="fluxdigest/worker:${STACK_RELEASE_ID}"
  export FLUXDIGEST_SCHEDULER_IMAGE="fluxdigest/scheduler:${STACK_RELEASE_ID}"
}

generate_credentials() {
  export http_proxy="${http_proxy:-${HTTP_PROXY:-}}"
  export https_proxy="${https_proxy:-${HTTPS_PROXY:-}}"
  export HTTP_PROXY="${HTTP_PROXY:-${http_proxy}}"
  export HTTPS_PROXY="${HTTPS_PROXY:-${https_proxy}}"
  local runtime_no_proxy
  runtime_no_proxy="$(merge_csv_values "${no_proxy:-}" "${NO_PROXY:-}" "localhost,127.0.0.1,::1,postgres,redis,miniflux,halo")"
  export no_proxy="${runtime_no_proxy}"
  export NO_PROXY="${runtime_no_proxy}"
  export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
  export GOSUMDB="${GOSUMDB:-sum.golang.google.cn}"
  export DOCKER_CONFIGURE_DAEMON_PROXY="${DOCKER_CONFIGURE_DAEMON_PROXY:-auto}"
  export DOCKER_SYSTEMD_DROPIN_DIR="${DOCKER_SYSTEMD_DROPIN_DIR:-${DEFAULT_DOCKER_SYSTEMD_DROPIN_DIR}}"
  export POSTGRES_IMAGE="${POSTGRES_IMAGE:-postgres:17}"
  export REDIS_IMAGE="${REDIS_IMAGE:-redis:7}"
  export MINIFLUX_IMAGE="${MINIFLUX_IMAGE:-ghcr.io/miniflux/miniflux:2.2.15}"
  export HALO_IMAGE="${HALO_IMAGE:-registry.fit2cloud.com/halo/halo:2.23}"

  export APP_HTTP_PORT="${FLUXDIGEST_HTTP_PORT}"
  export APP_REDIS_ADDR="redis:6379"
  export APP_JOB_API_KEY="${APP_JOB_API_KEY:-$(random_token 24)}"
  export APP_JOB_QUEUE="${APP_JOB_QUEUE:-default}"
  export APP_WORKER_CONCURRENCY="${APP_WORKER_CONCURRENCY:-6}"

  export APP_ADMIN_SESSION_SECRET="${APP_ADMIN_SESSION_SECRET:-$(random_token 32)}"
  export APP_SECRET_KEY="${APP_SECRET_KEY:-$(random_base64_token 24)}"
  export APP_ADMIN_BOOTSTRAP_USERNAME="${APP_ADMIN_BOOTSTRAP_USERNAME:-fluxdigest-admin}"
  export APP_ADMIN_BOOTSTRAP_PASSWORD="${APP_ADMIN_BOOTSTRAP_PASSWORD:-$(random_token 20)}"

  export POSTGRES_ROOT_USER="${POSTGRES_ROOT_USER:-postgres}"
  export POSTGRES_ROOT_PASSWORD="${POSTGRES_ROOT_PASSWORD:-$(random_token 24)}"
  export POSTGRES_DEFAULT_DB="${POSTGRES_DEFAULT_DB:-postgres}"

  export FLUXDIGEST_DB_NAME="${FLUXDIGEST_DB_NAME:-fluxdigest}"
  export FLUXDIGEST_DB_USER="${FLUXDIGEST_DB_USER:-fluxdigest}"
  export FLUXDIGEST_DB_PASSWORD="${FLUXDIGEST_DB_PASSWORD:-$(random_token 24)}"
  export APP_DATABASE_NAME="${FLUXDIGEST_DB_NAME}"
  export APP_DATABASE_USER="${FLUXDIGEST_DB_USER}"
  export APP_DATABASE_PASSWORD="${FLUXDIGEST_DB_PASSWORD}"
  export APP_DATABASE_DSN="postgres://${FLUXDIGEST_DB_USER}:${FLUXDIGEST_DB_PASSWORD}@postgres:5432/${FLUXDIGEST_DB_NAME}?sslmode=disable"

  export MINIFLUX_DB_NAME="${MINIFLUX_DB_NAME:-miniflux}"
  export MINIFLUX_DB_USER="${MINIFLUX_DB_USER:-miniflux}"
  export MINIFLUX_DB_PASSWORD="${MINIFLUX_DB_PASSWORD:-$(random_token 24)}"
  export MINIFLUX_ADMIN_USERNAME="${MINIFLUX_ADMIN_USERNAME:-admin}"
  export MINIFLUX_ADMIN_PASSWORD="${MINIFLUX_ADMIN_PASSWORD:-$(random_token 20)}"
  export MINIFLUX_API_KEY_DESCRIPTION="${MINIFLUX_API_KEY_DESCRIPTION:-FluxDigest Installer}"
  export APP_MINIFLUX_BASE_URL="${APP_MINIFLUX_BASE_URL:-}"
  export APP_MINIFLUX_AUTH_TOKEN="${APP_MINIFLUX_AUTH_TOKEN:-}"

  export HALO_DB_NAME="${HALO_DB_NAME:-halo}"
  export HALO_DB_USER="${HALO_DB_USER:-halo}"
  export HALO_DB_PASSWORD="${HALO_DB_PASSWORD:-$(random_token 24)}"
  export HALO_ADMIN_USERNAME="${HALO_ADMIN_USERNAME:-admin}"
  export HALO_ADMIN_PASSWORD="${HALO_ADMIN_PASSWORD:-$(random_token 20)}"
  export HALO_ADMIN_EMAIL="${HALO_ADMIN_EMAIL:-admin@fluxdigest.local}"
  export HALO_SITE_TITLE="${HALO_SITE_TITLE:-FluxDigest}"
  export HALO_PAT_NAME="${HALO_PAT_NAME:-FluxDigest Publisher}"
  export HALO_EXTERNAL_URL="${HALO_EXTERNAL_URL:-http://127.0.0.1:${HALO_HTTP_PORT}}"

  export APP_LLM_BASE_URL="${APP_LLM_BASE_URL:-}"
  export APP_LLM_API_KEY="${APP_LLM_API_KEY:-}"
  export APP_LLM_MODEL="${APP_LLM_MODEL:-MiniMax-M2.7}"
  export APP_LLM_FALLBACK_MODELS="${APP_LLM_FALLBACK_MODELS:-mimo-v2-pro}"
  export APP_LLM_TIMEOUT_MS="${APP_LLM_TIMEOUT_MS:-45000}"

  export APP_PUBLISH_CHANNEL="${APP_PUBLISH_CHANNEL:-markdown_export}"
  export APP_PUBLISH_HALO_BASE_URL="${APP_PUBLISH_HALO_BASE_URL:-}"
  export APP_PUBLISH_HALO_TOKEN="${APP_PUBLISH_HALO_TOKEN:-}"
  export APP_PUBLISH_OUTPUT_DIR="${APP_PUBLISH_OUTPUT_DIR:-/app/data/output}"

  setup_profile_service_blocks
  log_info "Credentials and environment variables generated"
}

preservable_env_key() {
  case "${1:-}" in
    STACK_PROFILE|STACK_PUBLIC_HOST|FLUXDIGEST_RELEASE_ID|FLUXDIGEST_API_IMAGE|FLUXDIGEST_WORKER_IMAGE|FLUXDIGEST_SCHEDULER_IMAGE|\
    http_proxy|https_proxy|HTTP_PROXY|HTTPS_PROXY|no_proxy|NO_PROXY|GOPROXY|GOSUMDB|DOCKER_CONFIGURE_DAEMON_PROXY|DOCKER_SYSTEMD_DROPIN_DIR|POSTGRES_IMAGE|REDIS_IMAGE|MINIFLUX_IMAGE|HALO_IMAGE|\
    APP_HTTP_PORT|APP_DATABASE_NAME|APP_DATABASE_USER|APP_DATABASE_PASSWORD|APP_DATABASE_DSN|APP_REDIS_ADDR|APP_JOB_API_KEY|APP_JOB_QUEUE|APP_WORKER_CONCURRENCY|\
    APP_ADMIN_SESSION_SECRET|APP_SECRET_KEY|APP_ADMIN_BOOTSTRAP_USERNAME|APP_ADMIN_BOOTSTRAP_PASSWORD|APP_MINIFLUX_BASE_URL|APP_MINIFLUX_AUTH_TOKEN|\
    APP_LLM_BASE_URL|APP_LLM_API_KEY|APP_LLM_MODEL|APP_LLM_FALLBACK_MODELS|APP_LLM_TIMEOUT_MS|\
    APP_PUBLISH_CHANNEL|APP_PUBLISH_HALO_BASE_URL|APP_PUBLISH_HALO_TOKEN|APP_PUBLISH_OUTPUT_DIR|\
    POSTGRES_ROOT_USER|POSTGRES_ROOT_PASSWORD|POSTGRES_DEFAULT_DB|\
    FLUXDIGEST_DB_NAME|FLUXDIGEST_DB_USER|FLUXDIGEST_DB_PASSWORD|\
    MINIFLUX_DB_NAME|MINIFLUX_DB_USER|MINIFLUX_DB_PASSWORD|MINIFLUX_ADMIN_USERNAME|MINIFLUX_ADMIN_PASSWORD|MINIFLUX_API_KEY_DESCRIPTION|\
    HALO_DB_NAME|HALO_DB_USER|HALO_DB_PASSWORD|HALO_ADMIN_USERNAME|HALO_ADMIN_PASSWORD|HALO_ADMIN_EMAIL|HALO_SITE_TITLE|HALO_PAT_NAME|HALO_EXTERNAL_URL)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

load_env_values_from_file() {
  local existing_env="${1:?}"
  local override_mode="${2:-preserve}"
  [[ -f "${existing_env}" ]] || return 0

  local loaded=0
  local line key value
  while IFS= read -r line || [[ -n "${line}" ]]; do
    line="${line%$'\r'}"
    [[ -n "${line}" ]] || continue
    [[ "${line}" =~ ^[[:space:]]*# ]] && continue
    [[ "${line}" == *=* ]] || continue

    key="${line%%=*}"
    value="${line#*=}"
    key="$(trim_spaces "${key}")"
    preservable_env_key "${key}" || continue

    if [[ "${override_mode}" == "override" ]]; then
      export "${key}=${value}"
      loaded=1
      continue
    fi

    if [[ "${key}" == "STACK_PROFILE" && "${STACK_PROFILE_EXPLICIT}" -eq 0 ]]; then
      export "${key}=${value}"
      loaded=1
      continue
    fi

    if [[ "${key}" == "STACK_PUBLIC_HOST" && "${STACK_PUBLIC_HOST_EXPLICIT}" -eq 0 ]]; then
      export "${key}=${value}"
      loaded=1
      continue
    fi

    if [[ -z "${!key+x}" ]]; then
      export "${key}=${value}"
      loaded=1
    fi
  done < "${existing_env}"

  if [[ "${loaded}" -eq 1 ]]; then
    log_info "Loaded stack env defaults from ${existing_env}"
  fi
}

load_existing_env_values() {
  load_env_values_from_file "${STACK_DIR}/.env" preserve
}

reload_current_env_values() {
  load_env_values_from_file "${STACK_DIR}/.env" override
}

current_public_host() {
  printf '%s' "${STACK_PUBLIC_HOST:-<host>}"
}

release_snapshot_dir() {
  local release_id="${1:?}"
  printf '%s\n' "${STACK_RELEASES_DIR}/${release_id}"
}

snapshot_release_state() {
  local release_id="${1:?}"
  local snapshot_dir
  snapshot_dir="$(release_snapshot_dir "${release_id}")"
  mkdir -p "${snapshot_dir}"
  cp "${STACK_DIR}/.env" "${snapshot_dir}/.env"
  cp "${STACK_DIR}/docker-compose.yml" "${snapshot_dir}/docker-compose.yml"
  if [[ -f "${STACK_DIR}/install-summary.txt" ]]; then
    cp "${STACK_DIR}/install-summary.txt" "${snapshot_dir}/install-summary.txt"
  fi
  printf '%s\n' "${release_id}" > "${STACK_RELEASES_DIR}/current"
  log_info "Snapshot saved for release ${release_id}"
}

restore_release_snapshot() {
  local release_id="${1:?}"
  local snapshot_dir
  snapshot_dir="$(release_snapshot_dir "${release_id}")"
  [[ -f "${snapshot_dir}/.env" ]] || fail "找不到 release 快照: ${release_id}"
  [[ -f "${snapshot_dir}/docker-compose.yml" ]] || fail "release 快照缺少 docker-compose.yml: ${release_id}"
  cp "${snapshot_dir}/.env" "${STACK_DIR}/.env"
  cp "${snapshot_dir}/docker-compose.yml" "${STACK_DIR}/docker-compose.yml"
  if [[ -f "${snapshot_dir}/install-summary.txt" ]]; then
    cp "${snapshot_dir}/install-summary.txt" "${STACK_DIR}/install-summary.txt"
  fi
  printf '%s\n' "${release_id}" > "${STACK_RELEASES_DIR}/current"
  log_info "Snapshot restored for release ${release_id}"
}

list_release_ids() {
  [[ -d "${STACK_RELEASES_DIR}" ]] || return 0
  find "${STACK_RELEASES_DIR}" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | grep -E '^[0-9]{14}$' | sort -r || true
}

current_release_id() {
  if [[ -n "${FLUXDIGEST_RELEASE_ID:-}" ]]; then
    printf '%s\n' "${FLUXDIGEST_RELEASE_ID}"
    return 0
  fi
  if [[ -f "${STACK_RELEASES_DIR}/current" ]]; then
    sed -n '1p' "${STACK_RELEASES_DIR}/current"
    return 0
  fi
  return 1
}

resolve_rollback_target_id() {
  if [[ -n "${STACK_RELEASE_TARGET_ID}" ]]; then
    local target_dir
    target_dir="$(release_snapshot_dir "${STACK_RELEASE_TARGET_ID}")"
    [[ -d "${target_dir}" ]] || fail "指定 release 不存在: ${STACK_RELEASE_TARGET_ID}"
    printf '%s\n' "${STACK_RELEASE_TARGET_ID}"
    return 0
  fi

  local current_id
  current_id="$(current_release_id || true)"
  [[ -n "${current_id}" ]] || fail "当前 release ID 不存在，无法自动回滚"

  local releases=()
  mapfile -t releases < <(list_release_ids)
  [[ "${#releases[@]}" -gt 0 ]] || fail "没有可用的历史 release"

  local index
  for index in "${!releases[@]}"; do
    if [[ "${releases[index]}" == "${current_id}" ]]; then
      local target_index=$((index + 1))
      [[ "${target_index}" -lt "${#releases[@]}" ]] || fail "当前 release 已是最旧版本，无法继续回滚"
      printf '%s\n' "${releases[target_index]}"
      return 0
    fi
  done

  fail "未在 release 列表中找到当前 release: ${current_id}"
}

escape_systemd_env_value() {
  printf '%s' "${1:-}" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

configure_docker_daemon_proxy_if_needed() {
  local mode="${DOCKER_CONFIGURE_DAEMON_PROXY:-auto}"
  case "${mode}" in
    auto|force|skip) ;;
    *) fail "DOCKER_CONFIGURE_DAEMON_PROXY 仅支持 auto | force | skip，当前: ${mode}" ;;
  esac

  if [[ "${mode}" == "skip" ]]; then
    log_info "Skipping Docker daemon proxy configuration by request"
    return 0
  fi

  local docker_http_proxy="${DOCKER_HTTP_PROXY:-${HTTP_PROXY:-${http_proxy:-}}}"
  local docker_https_proxy="${DOCKER_HTTPS_PROXY:-${HTTPS_PROXY:-${https_proxy:-${docker_http_proxy}}}}"
  local docker_no_proxy="${DOCKER_NO_PROXY:-${NO_PROXY:-${no_proxy:-localhost,127.0.0.1,::1}}}"

  if [[ -z "${docker_http_proxy}" && -z "${docker_https_proxy}" ]]; then
    log_info "No HTTP(S) proxy detected; skipping Docker daemon proxy configuration"
    return 0
  fi

  [[ -n "${docker_http_proxy}" ]] || docker_http_proxy="${docker_https_proxy}"
  [[ -n "${docker_https_proxy}" ]] || docker_https_proxy="${docker_http_proxy}"

  if ! command -v systemctl >/dev/null 2>&1; then
    if [[ "${mode}" == "force" ]]; then
      fail "需要 systemctl 来配置 Docker daemon 代理，但当前系统不可用"
    fi
    log_warn "systemctl 不可用，跳过 Docker daemon 代理配置"
    return 0
  fi

  local dropin_dir="${DOCKER_SYSTEMD_DROPIN_DIR:-${DEFAULT_DOCKER_SYSTEMD_DROPIN_DIR}}"
  local dropin_file="${dropin_dir}/http-proxy.conf"
  local tmp_file="${dropin_file}.tmp.$$"
  local escaped_http escaped_https escaped_no_proxy
  escaped_http="$(escape_systemd_env_value "${docker_http_proxy}")"
  escaped_https="$(escape_systemd_env_value "${docker_https_proxy}")"
  escaped_no_proxy="$(escape_systemd_env_value "${docker_no_proxy}")"

  mkdir -p "${dropin_dir}"
  cat > "${tmp_file}" <<EOF
[Service]
Environment="HTTP_PROXY=${escaped_http}"
Environment="HTTPS_PROXY=${escaped_https}"
Environment="NO_PROXY=${escaped_no_proxy}"
Environment="http_proxy=${escaped_http}"
Environment="https_proxy=${escaped_https}"
Environment="no_proxy=${escaped_no_proxy}"
EOF

  local changed=1
  if [[ -f "${dropin_file}" ]] && cmp -s "${tmp_file}" "${dropin_file}"; then
    changed=0
    rm -f "${tmp_file}"
    log_info "Docker daemon proxy drop-in already up to date: ${dropin_file}"
  else
    mv "${tmp_file}" "${dropin_file}"
    log_info "Docker daemon proxy drop-in written: ${dropin_file}"
  fi

  if [[ "${changed}" -eq 1 ]]; then
    systemctl daemon-reload
    systemctl restart docker

    local attempt
    for attempt in $(seq 1 20); do
      if docker info >/dev/null 2>&1; then
        log_info "Docker daemon restarted and is ready"
        return 0
      fi
      sleep 1
    done
    fail "Docker daemon restart completed but docker info did not recover in time"
  fi
}

required_external_images() {
  printf '%s\n' "${POSTGRES_IMAGE}" "${REDIS_IMAGE}"
  if profile_has_service "${STACK_PROFILE}" "miniflux"; then
    printf '%s\n' "${MINIFLUX_IMAGE}"
  fi
  if profile_has_service "${STACK_PROFILE}" "halo"; then
    printf '%s\n' "${HALO_IMAGE}"
  fi
}

prepull_external_images() {
  log_info "Pre-pulling external images"

  local image
  while IFS= read -r image; do
    [[ -n "${image}" ]] || continue
    if docker image inspect "${image}" >/dev/null 2>&1; then
      log_info "Using cached image: ${image}"
      continue
    fi
    log_info "Pulling image: ${image}"
    if ! docker pull "${image}"; then
      fail "外部镜像拉取失败: ${image}。请检查 Docker daemon 网络/代理配置，或通过环境变量覆盖镜像地址。"
    fi
  done < <(required_external_images)
}

build_fluxdigest_images() {
  log_info "Building FluxDigest images"
  docker compose \
    --project-directory "${STACK_DIR}" \
    --env-file "${STACK_DIR}/.env" \
    -f "${STACK_DIR}/docker-compose.yml" \
    build fluxdigest-api fluxdigest-worker fluxdigest-scheduler
}

start_compose_services() {
  local services=("$@")
  [[ "${#services[@]}" -gt 0 ]] || return 0

  docker compose \
    --project-directory "${STACK_DIR}" \
    --env-file "${STACK_DIR}/.env" \
    -f "${STACK_DIR}/docker-compose.yml" \
    up -d "${services[@]}"
}

start_selected_services() {
  local bootstrap_integrations="${1:-1}"

  log_info "Starting base services: postgres redis"
  start_compose_services postgres redis

  if profile_has_service "${STACK_PROFILE}" "miniflux"; then
    log_info "Starting Miniflux"
    start_compose_services miniflux
    wait_for_http_ok "http://127.0.0.1:${MINIFLUX_HTTP_PORT}/healthz" 60 2 || fail "Miniflux 健康检查失败"
    if [[ "${bootstrap_integrations}" -eq 1 ]]; then
      export MINIFLUX_BASE_URL="http://127.0.0.1:${MINIFLUX_HTTP_PORT}"
      export APP_MINIFLUX_BASE_URL="http://miniflux:8080"
      local miniflux_token
      if ! miniflux_token="$(bootstrap_miniflux)"; then
        fail "Miniflux bootstrap 失败"
      fi
      export APP_MINIFLUX_AUTH_TOKEN="${miniflux_token}"
      render_stack_files
    fi
  fi

  if profile_has_service "${STACK_PROFILE}" "halo"; then
    log_info "Starting Halo"
    start_compose_services halo
    wait_for_http_ok "http://127.0.0.1:${HALO_HTTP_PORT}/actuator/health" 90 3 || fail "Halo 健康检查失败"
    if [[ "${bootstrap_integrations}" -eq 1 ]]; then
      export HALO_BASE_URL="http://127.0.0.1:${HALO_HTTP_PORT}"
      export APP_PUBLISH_CHANNEL="halo"
      export APP_PUBLISH_HALO_BASE_URL="http://halo:8090"
      local halo_token
      if ! halo_token="$(bootstrap_halo)"; then
        fail "Halo bootstrap 失败"
      fi
      export APP_PUBLISH_HALO_TOKEN="${halo_token}"
      render_stack_files
    fi
  fi

  log_info "Starting FluxDigest services"
  start_compose_services fluxdigest-api fluxdigest-worker fluxdigest-scheduler
  wait_for_http_ok "http://127.0.0.1:${FLUXDIGEST_HTTP_PORT}/healthz" 60 2 || fail "FluxDigest API 健康检查失败"
}

write_install_summary() {
  local summary_path="${STACK_DIR}/install-summary.txt"
  local generated_at
  local display_host
  generated_at="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
  display_host="$(current_public_host)"

  render_summary_report 1 "${generated_at}" "${summary_path}" > "${summary_path}"

  chmod 600 "${summary_path}" || true
  log_info "Install summary written to ${summary_path}"
}

render_summary_report() {
  local include_secrets="${1:-1}"
  local generated_at="${2:-$(date -u +'%Y-%m-%dT%H:%M:%SZ')}"
  local summary_path="${3:-${STACK_DIR}/install-summary.txt}"
  local display_host
  display_host="$(current_public_host)"

  printf 'Stack Action: %s\n' "${STACK_ACTION}"
  printf 'Install Profile: %s\n' "${STACK_PROFILE}"
  printf 'Stack Directory: %s\n' "${STACK_DIR}"
  printf 'Generated At: %s\n\n' "${generated_at}"

  printf 'Release\n'
  printf -- '- Current Release: %s\n' "${FLUXDIGEST_RELEASE_ID:-unknown}"
  printf -- '- API Image: %s\n' "${FLUXDIGEST_API_IMAGE:-unknown}"
  printf -- '- Worker Image: %s\n' "${FLUXDIGEST_WORKER_IMAGE:-unknown}"
  printf -- '- Scheduler Image: %s\n\n' "${FLUXDIGEST_SCHEDULER_IMAGE:-unknown}"

  printf 'Service URLs\n'
  printf -- '- FluxDigest WebUI / API: http://%s:%s\n' "${display_host}" "${FLUXDIGEST_HTTP_PORT}"
  if profile_has_service "${STACK_PROFILE}" "miniflux"; then
    printf -- '- Miniflux: http://%s:%s\n' "${display_host}" "${MINIFLUX_HTTP_PORT}"
  else
    printf -- '- Miniflux: Not installed in this profile\n'
  fi
  if profile_has_service "${STACK_PROFILE}" "halo"; then
    printf -- '- Halo: http://%s:%s\n' "${display_host}" "${HALO_HTTP_PORT}"
  else
    printf -- '- Halo: Not installed in this profile\n'
  fi

  if [[ "${include_secrets}" -eq 1 ]]; then
    printf '\nFluxDigest Admin\n'
    printf -- '- Username: %s\n' "${APP_ADMIN_BOOTSTRAP_USERNAME}"
    printf -- '- Password: %s\n' "${APP_ADMIN_BOOTSTRAP_PASSWORD}"

    printf '\nMiniflux Admin\n'
    if profile_has_service "${STACK_PROFILE}" "miniflux"; then
      printf -- '- Username: %s\n' "${MINIFLUX_ADMIN_USERNAME}"
      printf -- '- Password: %s\n' "${MINIFLUX_ADMIN_PASSWORD}"
    else
      printf -- '- Not installed in this profile\n'
    fi

    printf '\nHalo Admin\n'
    if profile_has_service "${STACK_PROFILE}" "halo"; then
      printf -- '- Username: %s\n' "${HALO_ADMIN_USERNAME}"
      printf -- '- Password: %s\n' "${HALO_ADMIN_PASSWORD}"
      printf -- '- Email: %s\n' "${HALO_ADMIN_EMAIL}"
    else
      printf -- '- Not installed in this profile\n'
    fi
  else
    printf '\nCredentials\n'
    printf -- '- Sensitive credentials are stored in: %s\n' "${summary_path}"
    printf -- '- If this file is root-only, use: sudo cat %s\n' "${summary_path}"
  fi

  printf '\nPostgreSQL\n'
  printf -- '- Host: %s\n' "${display_host}"
  printf -- '- Port: %s\n' "${POSTGRES_HOST_PORT}"
  if [[ "${include_secrets}" -eq 1 ]]; then
    printf -- '- FluxDigest DB: %s / %s / %s\n' "${FLUXDIGEST_DB_NAME}" "${FLUXDIGEST_DB_USER}" "${FLUXDIGEST_DB_PASSWORD}"
    printf -- '- Miniflux DB: %s / %s / %s\n' "${MINIFLUX_DB_NAME}" "${MINIFLUX_DB_USER}" "${MINIFLUX_DB_PASSWORD}"
    printf -- '- Halo DB: %s / %s / %s\n' "${HALO_DB_NAME}" "${HALO_DB_USER}" "${HALO_DB_PASSWORD}"
  else
    printf -- '- FluxDigest DB: %s / %s / <redacted>\n' "${FLUXDIGEST_DB_NAME}" "${FLUXDIGEST_DB_USER}"
    printf -- '- Miniflux DB: %s / %s / <redacted>\n' "${MINIFLUX_DB_NAME}" "${MINIFLUX_DB_USER}"
    printf -- '- Halo DB: %s / %s / <redacted>\n' "${HALO_DB_NAME}" "${HALO_DB_USER}"
  fi

  printf '\nRedis\n'
  printf -- '- Host: %s\n' "${display_host}"
  printf -- '- Port: %s\n' "${REDIS_HOST_PORT}"

  printf '\nImages\n'
  printf -- '- PostgreSQL: %s\n' "${POSTGRES_IMAGE}"
  printf -- '- Redis: %s\n' "${REDIS_IMAGE}"
  printf -- '- Miniflux: %s\n' "$(if profile_has_service "${STACK_PROFILE}" "miniflux"; then printf '%s' "${MINIFLUX_IMAGE}"; else printf 'Not installed in this profile'; fi)"
  printf -- '- Halo: %s\n' "$(if profile_has_service "${STACK_PROFILE}" "halo"; then printf '%s' "${HALO_IMAGE}"; else printf 'Not installed in this profile'; fi)"

  printf '\nImportant Files\n'
  printf -- '- .env: %s\n' "${STACK_DIR}/.env"
  printf -- '- docker-compose.yml: %s\n' "${STACK_DIR}/docker-compose.yml"
  printf -- '- install-summary.txt: %s\n' "${summary_path}"
  printf -- '- releases/: %s\n' "${STACK_RELEASES_DIR}"

  printf '\nNext Steps\n'
  printf -- '- 1) 登录 FluxDigest WebUI\n'
  printf -- '- 2) 在 WebUI 中配置 LLM（Base URL / API Key / Model）\n'
  printf -- '- 3) 如包含 Miniflux / Halo，可分别登录后台确认状态\n'

  printf '\nOps Commands\n'
  printf -- '- cd %s && docker compose --env-file .env -f docker-compose.yml ps\n' "${STACK_DIR}"
  printf -- '- cd %s && docker compose --env-file .env -f docker-compose.yml logs -f fluxdigest-api\n' "${STACK_DIR}"
  printf -- '- cd %s && docker compose --env-file .env -f docker-compose.yml restart\n' "${STACK_DIR}"
}

print_install_summary_hint() {
  local display_host
  display_host="$(current_public_host)"
  cat <<EOF
安装完成。
- FluxDigest WebUI / API: http://${display_host}:${FLUXDIGEST_HTTP_PORT}
- Miniflux: $(if profile_has_service "${STACK_PROFILE}" "miniflux"; then printf 'http://%s:%s' "${display_host}" "${MINIFLUX_HTTP_PORT}"; else printf 'Not installed in this profile'; fi)
- Halo: $(if profile_has_service "${STACK_PROFILE}" "halo"; then printf 'http://%s:%s' "${display_host}" "${HALO_HTTP_PORT}"; else printf 'Not installed in this profile'; fi)
- 详细凭据与连接信息请查看: ${STACK_DIR}/install-summary.txt
EOF
}

run_install_like_action() {
  load_existing_env_values
  generate_credentials
  set_release_image_tags
  render_stack_files
  configure_docker_daemon_proxy_if_needed
  prepull_external_images
  build_fluxdigest_images
  start_selected_services 1
  write_install_summary
  snapshot_release_state "${FLUXDIGEST_RELEASE_ID}"
  print_install_summary_hint
}

run_rollback_action() {
  load_existing_env_values
  local target_release_id
  target_release_id="$(resolve_rollback_target_id)"
  restore_release_snapshot "${target_release_id}"
  reload_current_env_values
  set_stack_paths
  start_selected_services 0
  write_install_summary
  print_install_summary_hint
}

run_status_action() {
  load_existing_env_values
  local summary_path="${STACK_DIR}/install-summary.txt"
  if [[ -f "${summary_path}" ]]; then
    if cat "${summary_path}" 2>/dev/null; then
      return 0
    fi
    log_warn "install-summary.txt 不可读，输出脱敏状态摘要"
  else
    log_warn "install-summary.txt 不存在，输出即时状态摘要"
  fi

  render_summary_report 0
}

main() {
  parse_args "$@"
  if [[ "${STACK_SHOW_HELP}" -eq 1 ]]; then
    usage
    return 0
  fi

  log_info "Starting stack action: ${STACK_ACTION}"
  ensure_linux
  ensure_required_commands
  prepare_stack_dir

  case "${STACK_ACTION}" in
    install|upgrade)
      run_install_like_action
      ;;
    rollback)
      run_rollback_action
      ;;
    status)
      run_status_action
      ;;
  esac

  log_info "Stack action finished: ${STACK_ACTION}"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
