#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../scripts/common.sh"

TMP_DIR="$(mktemp -d)"
WORK_DIR="${TMP_DIR}/work"
STACK_DIR_REL="release-stack"
MOCK_BIN="${TMP_DIR}/mock-bin"
MOCK_DOCKER_LOG="${TMP_DIR}/docker.log"
trap 'rm -rf "${TMP_DIR}"' EXIT

mkdir -p "${MOCK_BIN}" "${WORK_DIR}"

cat > "${MOCK_BIN}/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'docker %s\n' "$*" >> "${MOCK_DOCKER_LOG:?}"
if [[ "${1:-}" == "compose" || "${1:-}" == "image" ]]; then
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
  awk -v n="${len}" 'BEGIN { for (i = 0; i < n * 2; i++) printf "b"; printf "\n" }'
  exit 0
fi
if [[ "${1:-}" == "rand" && "${2:-}" == "-base64" ]]; then
  bytes="${3:-24}"
  chars=$(( ((bytes + 2) / 3) * 4 ))
  awk -v n="${chars}" 'BEGIN { for (i = 0; i < n; i++) printf "B"; printf "\n" }'
  exit 0
fi
exit 1
EOF
chmod +x "${MOCK_BIN}/openssl"

export PATH="${MOCK_BIN}:${PATH}"
export MOCK_DOCKER_LOG

# shellcheck source=/dev/null
source "${SCRIPT_DIR}/../install.sh"

ensure_linux() { :; }
wait_for_http_ok() { return 0; }
bootstrap_miniflux() { printf 'miniflux-token\n'; }
bootstrap_halo() { printf 'halo-token\n'; }

export STACK_RELEASE_ID_OVERRIDE="20260415070001"
(
  cd "${WORK_DIR}"
  main --action install --profile fluxdigest-only --stack-dir "${STACK_DIR_REL}" --force
)

STACK_DIR="$(cd "${WORK_DIR}/${STACK_DIR_REL}" && pwd -P)"
grep -q '^FLUXDIGEST_RELEASE_ID=20260415070001$' "${STACK_DIR}/.env" || fail "初始 release ID 未写入 .env"
grep -q '^FLUXDIGEST_API_IMAGE=fluxdigest/api:20260415070001$' "${STACK_DIR}/.env" || fail "API 镜像 tag 未写入"
[[ -f "${STACK_DIR}/releases/20260415070001/.env" ]] || fail "初始 release 快照缺失"

export STACK_RELEASE_ID_OVERRIDE="20260415070002"
(
  cd "${WORK_DIR}"
  main --action upgrade --stack-dir "${STACK_DIR_REL}" --force
)

grep -q '^FLUXDIGEST_RELEASE_ID=20260415070002$' "${STACK_DIR}/.env" || fail "升级后 release ID 未更新"
[[ -f "${STACK_DIR}/releases/20260415070002/.env" ]] || fail "升级 release 快照缺失"

(
  cd "${WORK_DIR}"
  main --action rollback --stack-dir "${STACK_DIR_REL}" --release-id 20260415070001 --force
)

grep -q '^FLUXDIGEST_RELEASE_ID=20260415070001$' "${STACK_DIR}/.env" || fail "回滚后 release ID 未恢复"
grep -q 'Current Release: 20260415070001' "${STACK_DIR}/install-summary.txt" || fail "summary 未显示当前 release"
grep -Eq 'docker compose .*up -d fluxdigest-api fluxdigest-worker fluxdigest-scheduler' "${MOCK_DOCKER_LOG}" || fail "缺少 FluxDigest 服务启动命令"

log_info "install release actions smoke passed"
