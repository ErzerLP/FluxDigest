#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../scripts/common.sh"

TMP_DIR="$(mktemp -d)"
WORK_DIR="${TMP_DIR}/work"
STACK_DIR_REL="existing-env-stack"
MOCK_BIN="${TMP_DIR}/mock-bin"
MOCK_DOCKER_LOG="${TMP_DIR}/docker.log"
trap 'rm -rf "${TMP_DIR}"' EXIT

mkdir -p "${MOCK_BIN}" "${WORK_DIR}/${STACK_DIR_REL}"

cat > "${MOCK_BIN}/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
log_file="${MOCK_DOCKER_LOG:?}"
printf 'docker %s\n' "$*" >> "${log_file}"
if [[ "${1:-}" == "image" && "${2:-}" == "inspect" ]]; then
  exit 0
fi
exit 0
EOF
chmod +x "${MOCK_BIN}/docker"

cat > "${MOCK_BIN}/openssl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "rand" && "${2:-}" == "-hex" ]]; then
  len="${3:-16}"
  awk -v n="${len}" 'BEGIN { for (i = 0; i < n * 2; i++) printf "d"; printf "\n" }'
  exit 0
fi
if [[ "${1:-}" == "rand" && "${2:-}" == "-base64" ]]; then
  bytes="${3:-24}"
  chars=$(( ((bytes + 2) / 3) * 4 ))
  awk -v n="${chars}" 'BEGIN { for (i = 0; i < n; i++) printf "D"; printf "\n" }'
  exit 0
fi
echo "mock-openssl" >&2
exit 1
EOF
chmod +x "${MOCK_BIN}/openssl"

cat > "${WORK_DIR}/${STACK_DIR_REL}/.env" <<'EOF'
POSTGRES_ROOT_PASSWORD=existing-root-password
FLUXDIGEST_DB_PASSWORD=existing-flux-password
MINIFLUX_DB_PASSWORD=existing-miniflux-password
HALO_DB_PASSWORD=existing-halo-password
APP_ADMIN_BOOTSTRAP_PASSWORD=existing-admin-password
MINIFLUX_ADMIN_PASSWORD=existing-miniflux-admin
HALO_ADMIN_PASSWORD=existing-halo-admin
EOF

export PATH="${MOCK_BIN}:${PATH}"
export MOCK_DOCKER_LOG

# shellcheck source=/dev/null
source "${SCRIPT_DIR}/../install.sh"

ensure_linux() { :; }
wait_for_http_ok() { return 0; }
bootstrap_miniflux() { printf 'existing-miniflux-token\n'; }
bootstrap_halo() { printf 'existing-halo-token\n'; }

(
  cd "${WORK_DIR}"
  main --action install --profile full --stack-dir "${STACK_DIR_REL}" --force
)

env_file="${WORK_DIR}/${STACK_DIR_REL}/.env"
grep -q 'POSTGRES_ROOT_PASSWORD=existing-root-password' "${env_file}" || fail "未保留现有 PostgreSQL root 密码"
grep -q 'FLUXDIGEST_DB_PASSWORD=existing-flux-password' "${env_file}" || fail "未保留现有 FluxDigest DB 密码"
grep -q 'MINIFLUX_DB_PASSWORD=existing-miniflux-password' "${env_file}" || fail "未保留现有 Miniflux DB 密码"
grep -q 'HALO_DB_PASSWORD=existing-halo-password' "${env_file}" || fail "未保留现有 Halo DB 密码"
grep -q 'APP_ADMIN_BOOTSTRAP_PASSWORD=existing-admin-password' "${env_file}" || fail "未保留现有管理员密码"
grep -q 'MINIFLUX_ADMIN_PASSWORD=existing-miniflux-admin' "${env_file}" || fail "未保留现有 Miniflux 管理员密码"
grep -q 'HALO_ADMIN_PASSWORD=existing-halo-admin' "${env_file}" || fail "未保留现有 Halo 管理员密码"
grep -qE '^FLUXDIGEST_RELEASE_ID=[0-9]{14}$' "${env_file}" || fail "未写入 release ID"
grep -qE '^FLUXDIGEST_API_IMAGE=fluxdigest/api:[0-9]{14}$' "${env_file}" || fail "未写入 API 镜像 tag"
grep -qE '^FLUXDIGEST_WORKER_IMAGE=fluxdigest/worker:[0-9]{14}$' "${env_file}" || fail "未写入 Worker 镜像 tag"
grep -qE '^FLUXDIGEST_SCHEDULER_IMAGE=fluxdigest/scheduler:[0-9]{14}$' "${env_file}" || fail "未写入 Scheduler 镜像 tag"

log_info "install existing env smoke passed"
