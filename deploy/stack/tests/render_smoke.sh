#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../scripts/common.sh"

STACK_DIR="$(mktemp -d)"
trap 'rm -rf "${STACK_DIR}"' EXIT
export STACK_DIR

export STACK_PROFILE="fluxdigest-only"
export APP_HTTP_PORT="9090"
export APP_DATABASE_NAME="fluxdigest_smoke"
export APP_DATABASE_USER="fluxdigest_smoke"
export APP_DATABASE_PASSWORD="smoketest-pass"
export APP_DATABASE_DSN="postgres://fluxdigest_smoke:smoketest-pass@postgres:5432/fluxdigest_smoke?sslmode=disable"
export APP_REDIS_ADDR="redis:6379"
export APP_ADMIN_BOOTSTRAP_USERNAME="smoke-admin"
export APP_ADMIN_BOOTSTRAP_PASSWORD="smoke-password"

source "${SCRIPT_DIR}/../scripts/render.sh"

log_info "Rendering stack templates to ${STACK_DIR}"
render_stack_files

[[ -f "${STACK_DIR}/.env" ]] || fail "缺少渲染后的 .env"
grep -q "APP_HTTP_PORT=9090" "${STACK_DIR}/.env" || fail "APP_HTTP_PORT 未写入"
grep -q "APP_DATABASE_DSN" "${STACK_DIR}/.env" || fail "APP_DATABASE_DSN 未写入"
grep -q "APP_ADMIN_BOOTSTRAP_USERNAME=smoke-admin" "${STACK_DIR}/.env" || fail "管理员账号未写入"

saved_stack_services="${STACK_PROFILE_SERVICES:-}"
STACK_PROFILE_SERVICES="miniflux, halo"
if ! profile_has_service "fluxdigest-only" "halo"; then
  fail "profile_has_service 无法识别带空格的 STACK_PROFILE_SERVICES"
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
if grep -Eiq "miniflux:" "${STACK_DIR}/docker-compose.yml"; then
  fail "fluxdigest-only profile 不应包含 miniflux 服务定义"
fi
if grep -Eiq "halo:" "${STACK_DIR}/docker-compose.yml"; then
  fail "fluxdigest-only profile 不应包含 halo 服务定义"
fi

[[ -f "${STACK_DIR}/initdb/01-init-app-dbs.sql" ]] || fail "缺少 initdb SQL"
grep -q "CREATE DATABASE" "${STACK_DIR}/initdb/01-init-app-dbs.sql" || fail "数据库创建语句缺失"

grep -q "fluxdigest_smoke" "${STACK_DIR}/initdb/01-init-app-dbs.sql" || fail "SQL 未使用自定义数据库名"

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
