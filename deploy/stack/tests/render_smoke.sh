#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../scripts/common.sh"

STACK_DIR="$(mktemp -d)"
trap 'rm -rf "${STACK_DIR}"' EXIT
export STACK_DIR
export STACK_SOURCE_ROOT="/work/fluxdigest"
export STACK_INITDB_DIR="${STACK_DIR}/initdb"
export STACK_DATA_ROOT="${STACK_DIR}/data"
export STACK_POSTGRES_DATA_DIR="${STACK_DATA_ROOT}/postgres"
export STACK_REDIS_DATA_DIR="${STACK_DATA_ROOT}/redis"
export STACK_MINIFLUX_DATA_DIR="${STACK_DATA_ROOT}/miniflux"
export STACK_HALO_DATA_DIR="${STACK_DATA_ROOT}/halo"
export STACK_FLUXDIGEST_DATA_DIR="${STACK_DATA_ROOT}/fluxdigest"
export STACK_FLUXDIGEST_OUTPUT_DIR="${STACK_FLUXDIGEST_DATA_DIR}/output"
export STACK_LOG_DIR="${STACK_DIR}/logs"
export http_proxy="http://127.0.0.1:7890"
export https_proxy="http://127.0.0.1:7890"
export HTTP_PROXY="http://127.0.0.1:7890"
export HTTPS_PROXY="http://127.0.0.1:7890"
export GOPROXY="https://goproxy.cn,direct"
export GOSUMDB="sum.golang.google.cn"
export POSTGRES_IMAGE="postgres:17"
export REDIS_IMAGE="redis:7"
export MINIFLUX_IMAGE="miniflux/miniflux:latest"
export HALO_IMAGE="registry.fit2cloud.com/halo/halo:2.23"

export STACK_PROFILE="fluxdigest-only"
export APP_HTTP_PORT="18088"
export POSTGRES_ROOT_USER="postgres"
export POSTGRES_ROOT_PASSWORD="postgres-root-pass"
export POSTGRES_DEFAULT_DB="postgres"
export FLUXDIGEST_DB_NAME="fluxdigest_smoke"
export FLUXDIGEST_DB_USER="fluxdigest_smoke"
export FLUXDIGEST_DB_PASSWORD="smoketest-pass"
export MINIFLUX_DB_NAME="miniflux_smoke"
export MINIFLUX_DB_USER="miniflux_smoke"
export MINIFLUX_DB_PASSWORD="miniflux-pass"
export HALO_DB_NAME="halo_smoke"
export HALO_DB_USER="halo_smoke"
export HALO_DB_PASSWORD="halo-pass"
export APP_DATABASE_NAME="${FLUXDIGEST_DB_NAME}"
export APP_DATABASE_USER="${FLUXDIGEST_DB_USER}"
export APP_DATABASE_PASSWORD="${FLUXDIGEST_DB_PASSWORD}"
export APP_DATABASE_DSN="postgres://${FLUXDIGEST_DB_USER}:${FLUXDIGEST_DB_PASSWORD}@postgres:5432/${FLUXDIGEST_DB_NAME}?sslmode=disable"
export APP_REDIS_ADDR="redis:6379"
export APP_JOB_API_KEY="job-api-key"
export APP_JOB_QUEUE="default"
export APP_WORKER_CONCURRENCY="6"
export APP_ADMIN_SESSION_SECRET="admin-session-secret"
export APP_SECRET_KEY="app-secret-key"
export APP_ADMIN_BOOTSTRAP_USERNAME="smoke-admin"
export APP_ADMIN_BOOTSTRAP_PASSWORD="smoke-password"
export APP_MINIFLUX_BASE_URL=""
export APP_MINIFLUX_AUTH_TOKEN=""
export APP_LLM_BASE_URL=""
export APP_LLM_API_KEY=""
export APP_LLM_MODEL="MiniMax-M2.7"
export APP_LLM_FALLBACK_MODELS="mimo-v2-pro"
export APP_LLM_TIMEOUT_MS="45000"
export APP_PUBLISH_CHANNEL="markdown_export"
export APP_PUBLISH_HALO_BASE_URL=""
export APP_PUBLISH_HALO_TOKEN=""
export APP_PUBLISH_OUTPUT_DIR="/app/data/output"
export MINIFLUX_ADMIN_USERNAME="admin"
export MINIFLUX_ADMIN_PASSWORD="miniflux-admin-pass"
export MINIFLUX_API_KEY_DESCRIPTION="FluxDigest Installer"
export HALO_ADMIN_USERNAME="admin"
export HALO_ADMIN_PASSWORD="halo-admin-pass"
export HALO_ADMIN_EMAIL="admin@fluxdigest.local"
export HALO_PAT_NAME="FluxDigest Publisher"
export HALO_EXTERNAL_URL="http://127.0.0.1:28090"

source "${SCRIPT_DIR}/../scripts/render.sh"

log_info "Rendering stack templates to ${STACK_DIR}"
render_stack_files

[[ -f "${STACK_DIR}/.env" ]] || fail "缺少渲染后的 .env"
grep -q "APP_HTTP_PORT=18088" "${STACK_DIR}/.env" || fail "APP_HTTP_PORT 未写入"
grep -q "APP_DATABASE_DSN" "${STACK_DIR}/.env" || fail "APP_DATABASE_DSN 未写入"
grep -q "APP_ADMIN_BOOTSTRAP_USERNAME=smoke-admin" "${STACK_DIR}/.env" || fail "管理员账号未写入"
grep -q "APP_JOB_API_KEY=job-api-key" "${STACK_DIR}/.env" || fail "APP_JOB_API_KEY 未写入"
grep -q "http_proxy=http://127.0.0.1:7890" "${STACK_DIR}/.env" || fail "http_proxy 未写入"
grep -q "GOPROXY=https://goproxy.cn,direct" "${STACK_DIR}/.env" || fail "GOPROXY 未写入"
grep -q "POSTGRES_IMAGE=postgres:17" "${STACK_DIR}/.env" || fail "POSTGRES_IMAGE 未写入"
grep -q "MINIFLUX_IMAGE=miniflux/miniflux:latest" "${STACK_DIR}/.env" || fail "MINIFLUX_IMAGE 未写入"

saved_stack_services="${STACK_PROFILE_SERVICES:-}"
STACK_PROFILE_SERVICES="miniflux, halo"
if profile_has_service "fluxdigest-only" "halo"; then
  fail "固定 profile 不应被 STACK_PROFILE_SERVICES 污染"
fi
STACK_PROFILE_SERVICES="miniflux halo"
if ! profile_has_service "miniflux halo" "halo"; then
  fail "profile_has_service 无法识别含空格的自定义 profile"
fi
if [[ -n "${saved_stack_services}" ]]; then
  export STACK_PROFILE_SERVICES="${saved_stack_services}"
else
  unset STACK_PROFILE_SERVICES
fi

[[ -f "${STACK_DIR}/docker-compose.yml" ]] || fail "缺少 docker-compose.yml"
grep -q "fluxdigest-api" "${STACK_DIR}/docker-compose.yml" || fail "fluxdigest-api 服务未出现在 compose"
grep -q "HTTP_PROXY: \${HTTP_PROXY}" "${STACK_DIR}/docker-compose.yml" || fail "compose 未透传构建代理"
grep -q "GOPROXY: \${GOPROXY}" "${STACK_DIR}/docker-compose.yml" || fail "compose 未透传 Go 代理"
grep -q "image: \${POSTGRES_IMAGE}" "${STACK_DIR}/docker-compose.yml" || fail "compose 未参数化 postgres 镜像"
grep -q "image: \${REDIS_IMAGE}" "${STACK_DIR}/docker-compose.yml" || fail "compose 未参数化 redis 镜像"
if grep -Eiq "miniflux:" "${STACK_DIR}/docker-compose.yml"; then
  fail "fluxdigest-only profile 不应包含 miniflux 服务定义"
fi
if grep -Eiq "halo:" "${STACK_DIR}/docker-compose.yml"; then
  fail "fluxdigest-only profile 不应包含 halo 服务定义"
fi

[[ -f "${STACK_DIR}/initdb/01-init-app-dbs.sql" ]] || fail "缺少 initdb SQL"
grep -q "CREATE DATABASE" "${STACK_DIR}/initdb/01-init-app-dbs.sql" || fail "数据库创建语句缺失"

grep -q "fluxdigest_smoke" "${STACK_DIR}/initdb/01-init-app-dbs.sql" || fail "SQL 未使用 FluxDigest 数据库名"
grep -q "miniflux_smoke" "${STACK_DIR}/initdb/01-init-app-dbs.sql" || fail "SQL 未使用 Miniflux 数据库名"
grep -q "halo_smoke" "${STACK_DIR}/initdb/01-init-app-dbs.sql" || fail "SQL 未使用 Halo 数据库名"

if command -v docker >/dev/null 2>&1; then
  log_info "Validating compose manifest with docker compose"
  sanitized_compose="${STACK_DIR}/docker-compose.sanitized.yml"
  grep -Fv "__MINIFLUX_SERVICE_BLOCK__" "${STACK_DIR}/docker-compose.yml" | \
    grep -Fv "__HALO_SERVICE_BLOCK__" > "${sanitized_compose}"
  [[ -s "${sanitized_compose}" ]] || fail "清理后的 compose 内容为空"
  docker compose -f "${sanitized_compose}" config >/dev/null
  rm -f "${sanitized_compose}"
else
  log_warn "docker 未安装，跳过 compose config 校验"
fi
