#!/usr/bin/env bash

set -euo pipefail

INSTALL_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STACK_SOURCE_ROOT="$(cd "${INSTALL_SCRIPT_DIR}/../.." && pwd)"
STACK_PROFILE="${STACK_PROFILE:-full}"
STACK_DIR="${STACK_DIR:-/opt/fluxdigest-stack}"

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
  --profile <name>   Stack profile: full | fluxdigest-miniflux | fluxdigest-halo | fluxdigest-only
  --stack-dir <dir>  Target stack directory (default: /opt/fluxdigest-stack)
  --force            Allow overwriting existing generated files
  -h, --help         Show this help
EOF
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --profile)
        [[ $# -ge 2 ]] || fail "--profile 缺少参数"
        STACK_PROFILE="$2"
        shift 2
        ;;
      --stack-dir)
        [[ $# -ge 2 ]] || fail "--stack-dir 缺少参数"
        STACK_DIR="$2"
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
  for cmd in docker openssl sed awk curl; do
    require_cmd "${cmd}"
  done

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

set_stack_paths() {
  export STACK_SOURCE_ROOT
  export STACK_PROFILE

  export STACK_INITDB_DIR="${STACK_DIR}/initdb"
  export STACK_DATA_ROOT="${STACK_DIR}/data"
  export STACK_POSTGRES_DATA_DIR="${STACK_DATA_ROOT}/postgres"
  export STACK_REDIS_DATA_DIR="${STACK_DATA_ROOT}/redis"
  export STACK_MINIFLUX_DATA_DIR="${STACK_DATA_ROOT}/miniflux"
  export STACK_HALO_DATA_DIR="${STACK_DATA_ROOT}/halo"
  export STACK_FLUXDIGEST_DATA_DIR="${STACK_DATA_ROOT}/fluxdigest"
  export STACK_FLUXDIGEST_OUTPUT_DIR="${STACK_FLUXDIGEST_DATA_DIR}/output"
  export STACK_LOG_DIR="${STACK_DIR}/logs"
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
  if [[ "${STACK_FORCE}" -ne 1 ]]; then
    for file in "${key_files[@]}"; do
      if [[ -e "${file}" ]]; then
        fail "目标目录已存在关键文件，使用 --force 允许覆盖: ${file}"
      fi
    done
  fi

  mkdir -p \
    "${STACK_INITDB_DIR}" \
    "${STACK_POSTGRES_DATA_DIR}" \
    "${STACK_REDIS_DATA_DIR}" \
    "${STACK_MINIFLUX_DATA_DIR}" \
    "${STACK_HALO_DATA_DIR}" \
    "${STACK_FLUXDIGEST_OUTPUT_DIR}" \
    "${STACK_LOG_DIR}"

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

generate_credentials() {
  export http_proxy="${http_proxy:-${HTTP_PROXY:-}}"
  export https_proxy="${https_proxy:-${HTTPS_PROXY:-}}"
  export HTTP_PROXY="${HTTP_PROXY:-${http_proxy}}"
  export HTTPS_PROXY="${HTTPS_PROXY:-${https_proxy}}"
  export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
  export GOSUMDB="${GOSUMDB:-sum.golang.google.cn}"
  export DOCKER_CONFIGURE_DAEMON_PROXY="${DOCKER_CONFIGURE_DAEMON_PROXY:-auto}"
  export DOCKER_SYSTEMD_DROPIN_DIR="${DOCKER_SYSTEMD_DROPIN_DIR:-${DEFAULT_DOCKER_SYSTEMD_DROPIN_DIR}}"
  export POSTGRES_IMAGE="${POSTGRES_IMAGE:-postgres:17}"
  export REDIS_IMAGE="${REDIS_IMAGE:-redis:7}"
  export MINIFLUX_IMAGE="${MINIFLUX_IMAGE:-miniflux/miniflux:latest}"
  export HALO_IMAGE="${HALO_IMAGE:-registry.fit2cloud.com/halo/halo:2.23}"

  export APP_HTTP_PORT="${FLUXDIGEST_HTTP_PORT}"
  export APP_REDIS_ADDR="redis:6379"
  export APP_JOB_API_KEY="${APP_JOB_API_KEY:-$(random_token 24)}"
  export APP_JOB_QUEUE="${APP_JOB_QUEUE:-default}"
  export APP_WORKER_CONCURRENCY="${APP_WORKER_CONCURRENCY:-6}"

  export APP_ADMIN_SESSION_SECRET="${APP_ADMIN_SESSION_SECRET:-$(random_token 32)}"
  export APP_SECRET_KEY="${APP_SECRET_KEY:-$(random_token 32)}"
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

selected_services() {
  local services=(postgres redis fluxdigest-api fluxdigest-worker fluxdigest-scheduler)
  if profile_has_service "${STACK_PROFILE}" "miniflux"; then
    services+=(miniflux)
  fi
  if profile_has_service "${STACK_PROFILE}" "halo"; then
    services+=(halo)
  fi
  printf '%s\n' "${services[@]}"
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

bootstrap_halo() {
  log_info "Halo bootstrap placeholder (will be implemented in Task 5)"
}

start_selected_services() {
  log_info "Starting base services: postgres redis"
  start_compose_services postgres redis

  if profile_has_service "${STACK_PROFILE}" "miniflux"; then
    log_info "Starting Miniflux"
    start_compose_services miniflux
    wait_for_http_ok "http://127.0.0.1:${MINIFLUX_HTTP_PORT}/healthz" 60 2 || fail "Miniflux 健康检查失败"
    export MINIFLUX_BASE_URL="http://127.0.0.1:${MINIFLUX_HTTP_PORT}"
    export APP_MINIFLUX_BASE_URL="http://miniflux:8080"
    export APP_MINIFLUX_AUTH_TOKEN
    APP_MINIFLUX_AUTH_TOKEN="$(bootstrap_miniflux)"
    render_stack_files
  fi

  if profile_has_service "${STACK_PROFILE}" "halo"; then
    log_info "Starting Halo"
    start_compose_services halo
    wait_for_http_ok "http://127.0.0.1:${HALO_HTTP_PORT}/actuator/health" 90 3 || fail "Halo 健康检查失败"
    export HALO_BASE_URL="http://127.0.0.1:${HALO_HTTP_PORT}"
    export APP_PUBLISH_CHANNEL="halo"
    export APP_PUBLISH_HALO_BASE_URL="http://halo:8090"
    export APP_PUBLISH_HALO_TOKEN
    APP_PUBLISH_HALO_TOKEN="$(bootstrap_halo)"
    render_stack_files
  fi

  log_info "Starting FluxDigest services"
  start_compose_services fluxdigest-api fluxdigest-worker fluxdigest-scheduler
  wait_for_http_ok "http://127.0.0.1:${FLUXDIGEST_HTTP_PORT}/healthz" 60 2 || fail "FluxDigest API 健康检查失败"
}

write_install_summary() {
  local summary_path="${STACK_DIR}/install-summary.txt"
  local generated_at
  generated_at="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"

  {
    printf 'Install Profile: %s\n' "${STACK_PROFILE}"
    printf 'Stack Directory: %s\n' "${STACK_DIR}"
    printf 'Generated At: %s\n\n' "${generated_at}"

    printf 'Service URLs\n'
    printf -- '- FluxDigest WebUI / API: http://<host>:%s\n' "${FLUXDIGEST_HTTP_PORT}"
    if profile_has_service "${STACK_PROFILE}" "miniflux"; then
      printf -- '- Miniflux: http://<host>:%s\n' "${MINIFLUX_HTTP_PORT}"
    else
      printf -- '- Miniflux: Not installed in this profile\n'
    fi
    if profile_has_service "${STACK_PROFILE}" "halo"; then
      printf -- '- Halo: http://<host>:%s\n' "${HALO_HTTP_PORT}"
    else
      printf -- '- Halo: Not installed in this profile\n'
    fi

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

    printf '\nPostgreSQL\n'
    printf -- '- Host: <host>\n'
    printf -- '- Port: %s\n' "${POSTGRES_HOST_PORT}"
    printf -- '- FluxDigest DB: %s / %s / %s\n' "${FLUXDIGEST_DB_NAME}" "${FLUXDIGEST_DB_USER}" "${FLUXDIGEST_DB_PASSWORD}"
    printf -- '- Miniflux DB: %s / %s / %s\n' "${MINIFLUX_DB_NAME}" "${MINIFLUX_DB_USER}" "${MINIFLUX_DB_PASSWORD}"
    printf -- '- Halo DB: %s / %s / %s\n' "${HALO_DB_NAME}" "${HALO_DB_USER}" "${HALO_DB_PASSWORD}"

    printf '\nRedis\n'
    printf -- '- Host: <host>\n'
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

    printf '\nNext Steps\n'
    printf -- '- 1) 登录 FluxDigest WebUI\n'
    printf -- '- 2) 在 WebUI 中配置 LLM（Base URL / API Key / Model）\n'
    printf -- '- 3) 如包含 Miniflux / Halo，可分别登录后台确认状态\n'

    printf '\nOps Commands\n'
    printf -- '- cd %s && docker compose --env-file .env -f docker-compose.yml ps\n' "${STACK_DIR}"
    printf -- '- cd %s && docker compose --env-file .env -f docker-compose.yml logs -f fluxdigest-api\n' "${STACK_DIR}"
    printf -- '- cd %s && docker compose --env-file .env -f docker-compose.yml restart\n' "${STACK_DIR}"
  } > "${summary_path}"

  chmod 600 "${summary_path}" || true
  log_info "Install summary written to ${summary_path}"
}

print_install_summary_hint() {
  cat <<EOF
安装完成。
- FluxDigest WebUI / API: http://<host>:${FLUXDIGEST_HTTP_PORT}
- Miniflux: $(if profile_has_service "${STACK_PROFILE}" "miniflux"; then printf 'http://<host>:%s' "${MINIFLUX_HTTP_PORT}"; else printf 'Not installed in this profile'; fi)
- Halo: $(if profile_has_service "${STACK_PROFILE}" "halo"; then printf 'http://<host>:%s' "${HALO_HTTP_PORT}"; else printf 'Not installed in this profile'; fi)
- 详细凭据与连接信息请查看: ${STACK_DIR}/install-summary.txt
EOF
}

main() {
  parse_args "$@"
  if [[ "${STACK_SHOW_HELP}" -eq 1 ]]; then
    usage
    return 0
  fi

  log_info "Starting stack installation"
  ensure_linux
  ensure_required_commands
  prepare_stack_dir
  generate_credentials
  render_stack_files
  configure_docker_daemon_proxy_if_needed
  prepull_external_images
  build_fluxdigest_images
  start_selected_services
  write_install_summary
  print_install_summary_hint

  log_info "Installation finished. Summary: ${STACK_DIR}/install-summary.txt"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
