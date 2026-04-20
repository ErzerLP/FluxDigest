#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../scripts/common.sh"

TMP_DIR="$(mktemp -d)"
WORK_DIR="${TMP_DIR}/work"
STACK_DIR_REL="status-stack"
MOCK_BIN="${TMP_DIR}/mock-bin"
MOCK_DOCKER_LOG="${TMP_DIR}/docker.log"
STATUS_STDOUT="${TMP_DIR}/status.stdout"
STATUS_STDERR="${TMP_DIR}/status.stderr"
REAL_CAT="/usr/bin/cat"
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
  awk -v n="${len}" 'BEGIN { for (i = 0; i < n * 2; i++) printf "c"; printf "\n" }'
  exit 0
fi
if [[ "${1:-}" == "rand" && "${2:-}" == "-base64" ]]; then
  bytes="${3:-24}"
  chars=$(( ((bytes + 2) / 3) * 4 ))
  awk -v n="${chars}" 'BEGIN { for (i = 0; i < n; i++) printf "C"; printf "\n" }'
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

export STACK_RELEASE_ID_OVERRIDE="20260415080001"
(
  cd "${WORK_DIR}"
  main --action install --profile fluxdigest-only --stack-dir "${STACK_DIR_REL}" --host 192.168.50.10 --force
)

STACK_DIR="$(cd "${WORK_DIR}/${STACK_DIR_REL}" && pwd -P)"

cat() {
  if [[ "${1:-}" == "${STACK_DIR}/install-summary.txt" ]]; then
    echo "cat: ${STACK_DIR}/install-summary.txt: Permission denied" >&2
    return 1
  fi
  "${REAL_CAT}" "$@"
}

(
  cd "${WORK_DIR}"
  main --action status --stack-dir "${STACK_DIR_REL}" > "${STATUS_STDOUT}" 2> "${STATUS_STDERR}"
)

grep -q 'Current Release: 20260415080001' "${STATUS_STDOUT}" || fail "status 输出缺少当前 release"
grep -q 'FluxDigest WebUI / API: http://192.168.50.10:18088' "${STATUS_STDOUT}" || fail "status 输出缺少访问地址"
grep -q 'Sensitive credentials are stored in:' "${STATUS_STDOUT}" || fail "status 输出缺少敏感凭据提示"
grep -q '<redacted>' "${STATUS_STDOUT}" || fail "status 输出未对敏感字段脱敏"
