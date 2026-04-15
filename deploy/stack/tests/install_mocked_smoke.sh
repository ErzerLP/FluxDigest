#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../scripts/common.sh"

TMP_DIR="$(mktemp -d)"
WORK_DIR="${TMP_DIR}/work"
STACK_DIR_REL="relative-stack"
MOCK_BIN="${TMP_DIR}/mock-bin"
MOCK_DOCKER_LOG="${TMP_DIR}/docker.log"
trap 'rm -rf "${TMP_DIR}"' EXIT

mkdir -p "${MOCK_BIN}"
mkdir -p "${WORK_DIR}"

cat > "${MOCK_BIN}/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
log_file="${MOCK_DOCKER_LOG:?}"
printf 'docker %s\n' "$*" >> "${log_file}"
if [[ "${1:-}" == "compose" ]]; then
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
  awk -v n="${len}" 'BEGIN { for (i = 0; i < n * 2; i++) printf "a"; printf "\n" }'
  exit 0
fi
if [[ "${1:-}" == "rand" && "${2:-}" == "-base64" ]]; then
  bytes="${3:-24}"
  chars=$(( ((bytes + 2) / 3) * 4 ))
  awk -v n="${chars}" 'BEGIN { for (i = 0; i < n; i++) printf "A"; printf "\n" }'
  exit 0
fi
echo "mock-openssl" >&2
exit 1
EOF
chmod +x "${MOCK_BIN}/openssl"

export PATH="${MOCK_BIN}:${PATH}"
export MOCK_DOCKER_LOG

# shellcheck source=/dev/null
source "${SCRIPT_DIR}/../install.sh"

[[ "${STACK_DIR}" == "/opt/fluxdigest-stack" ]] || fail "source install.sh 后默认 STACK_DIR 不正确: ${STACK_DIR}"

# 覆写高耦合步骤，避免依赖真实宿主环境
ensure_linux() { :; }
wait_for_http_ok() { return 0; }
build_fluxdigest_images() {
  docker compose --project-directory "${STACK_DIR}" -f "${STACK_DIR}/docker-compose.yml" --env-file "${STACK_DIR}/.env" build fluxdigest-api fluxdigest-worker fluxdigest-scheduler
}
bootstrap_miniflux() { :; }
bootstrap_halo() { :; }

(
  cd "${WORK_DIR}"
  export STACK_PROFILE_SERVICES="miniflux,halo"
  main --action install --profile fluxdigest-only --stack-dir "${STACK_DIR_REL}" --force
)

STACK_DIR="$(cd "${WORK_DIR}/${STACK_DIR_REL}" && pwd -P)"

summary_file="${STACK_DIR}/install-summary.txt"
env_file="${STACK_DIR}/.env"
[[ -f "${summary_file}" ]] || fail "install-summary.txt 未生成"
[[ -f "${env_file}" ]] || fail ".env 未生成"
grep -q "Install Profile: fluxdigest-only" "${summary_file}" || fail "summary 缺少 profile"
grep -q "FluxDigest WebUI / API: http://<host>:18088" "${summary_file}" || fail "summary 缺少 FluxDigest URL"
grep -qE '^- \.env: /' "${summary_file}" || fail "summary 的 .env 路径不是绝对路径"
grep -qE '^- docker-compose.yml: /' "${summary_file}" || fail "summary 的 docker-compose.yml 路径不是绝对路径"
grep -qE '^- install-summary.txt: /' "${summary_file}" || fail "summary 的 install-summary.txt 路径不是绝对路径"

app_secret_key="$(sed -n 's/^APP_SECRET_KEY=//p' "${env_file}" | head -n 1)"
[[ -n "${app_secret_key}" ]] || fail "APP_SECRET_KEY 未写入 .env"
[[ "${#app_secret_key}" -eq 32 ]] || fail "APP_SECRET_KEY 长度应为 32 字节，实际为 ${#app_secret_key}"
grep -qE '^FLUXDIGEST_RELEASE_ID=[0-9]{14}$' "${env_file}" || fail "FLUXDIGEST_RELEASE_ID 未写入 .env"
grep -qE '^FLUXDIGEST_API_IMAGE=fluxdigest/api:[0-9]{14}$' "${env_file}" || fail "FLUXDIGEST_API_IMAGE 未写入 .env"
grep -qE '^FLUXDIGEST_WORKER_IMAGE=fluxdigest/worker:[0-9]{14}$' "${env_file}" || fail "FLUXDIGEST_WORKER_IMAGE 未写入 .env"
grep -qE '^FLUXDIGEST_SCHEDULER_IMAGE=fluxdigest/scheduler:[0-9]{14}$' "${env_file}" || fail "FLUXDIGEST_SCHEDULER_IMAGE 未写入 .env"

grep -Eq 'docker compose .*up -d postgres redis' "${MOCK_DOCKER_LOG}" || fail "mock docker 日志缺少基础服务 up 命令"
grep -Eq 'docker compose .*up -d fluxdigest-api fluxdigest-worker fluxdigest-scheduler' "${MOCK_DOCKER_LOG}" || fail "mock docker 日志缺少 FluxDigest 服务 up 命令"
if grep -Eq 'docker compose .*up -d .*miniflux|docker compose .*up -d .*halo' "${MOCK_DOCKER_LOG}"; then
  fail "fluxdigest-only 被 STACK_PROFILE_SERVICES 污染，出现了可选服务"
fi

log_info "install mocked smoke passed"
