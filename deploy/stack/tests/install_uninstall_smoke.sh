#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../scripts/common.sh"

TMP_DIR="$(mktemp -d)"
WORK_DIR="${TMP_DIR}/work"
STACK_DIR_REL="uninstall-stack"
MOCK_BIN="${TMP_DIR}/mock-bin"
MOCK_DOCKER_LOG="${TMP_DIR}/docker.log"
trap 'rm -rf "${TMP_DIR}"' EXIT

mkdir -p "${MOCK_BIN}" "${WORK_DIR}"

cat > "${MOCK_BIN}/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'docker %s\n' "$*" >> "${MOCK_DOCKER_LOG:?}"
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
exit 1
EOF
chmod +x "${MOCK_BIN}/openssl"

export PATH="${MOCK_BIN}:${PATH}"
export MOCK_DOCKER_LOG

# shellcheck source=/dev/null
source "${SCRIPT_DIR}/../install.sh"

ensure_linux() { :; }
wait_for_http_ok() { return 0; }
bootstrap_miniflux() { :; }
bootstrap_halo() { :; }

(
  cd "${WORK_DIR}"
  main --action install --profile fluxdigest-only --stack-dir "${STACK_DIR_REL}" --force
)

STACK_DIR="$(cd "${WORK_DIR}/${STACK_DIR_REL}" && pwd -P)"
[[ -f "${STACK_DIR}/.env" ]] || fail "安装后缺少 .env"
mkdir -p "${STACK_DIR}/data/fluxdigest"
printf 'keep me\n' > "${STACK_DIR}/data/fluxdigest/sample.txt"

(
  cd "${WORK_DIR}"
  main --action uninstall --stack-dir "${STACK_DIR_REL}" --force
)

[[ -f "${STACK_DIR}/data/fluxdigest/sample.txt" ]] || fail "默认卸载应保留数据目录"
grep -Eq 'docker compose .*down --remove-orphans' "${MOCK_DOCKER_LOG}" || fail "缺少 compose down"

(
  cd "${WORK_DIR}"
  main --action uninstall --stack-dir "${STACK_DIR_REL}" --force --purge-data
)

[[ ! -d "${STACK_DIR}" ]] || fail "purge-data 卸载后应删除 stack 目录"

log_info "install uninstall smoke passed"
