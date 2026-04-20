#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../scripts/common.sh"

TMP_DIR="$(mktemp -d)"
WORK_DIR="${TMP_DIR}/work"
STACK_DIR_REL="cached-stack"
MOCK_BIN="${TMP_DIR}/mock-bin"
MOCK_DOCKER_LOG="${TMP_DIR}/docker.log"
trap 'rm -rf "${TMP_DIR}"' EXIT

mkdir -p "${MOCK_BIN}" "${WORK_DIR}"

cat > "${MOCK_BIN}/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
log_file="${MOCK_DOCKER_LOG:?}"
printf 'docker %s\n' "$*" >> "${log_file}"
if [[ "${1:-}" == "image" && "${2:-}" == "inspect" && "${3:-}" == "cached.registry/miniflux:minicache" ]]; then
  exit 0
fi
if [[ "${1:-}" == "pull" && "${2:-}" == "cached.registry/miniflux:minicache" ]]; then
  echo "cached image should not be pulled" >&2
  exit 99
fi
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
echo "mock-openssl" >&2
exit 1
EOF
chmod +x "${MOCK_BIN}/openssl"

export PATH="${MOCK_BIN}:${PATH}"
export MOCK_DOCKER_LOG
export MINIFLUX_IMAGE="cached.registry/miniflux:minicache"

# shellcheck source=/dev/null
source "${SCRIPT_DIR}/../install.sh"

ensure_linux() { :; }
wait_for_http_ok() { return 0; }
bootstrap_miniflux() { printf 'cached-token\n'; }
bootstrap_halo() { printf 'cached-halo-token\n'; }

(
  cd "${WORK_DIR}"
  main --profile fluxdigest-miniflux --stack-dir "${STACK_DIR_REL}" --force
)

grep -Eq 'docker image inspect cached.registry/miniflux:minicache' "${MOCK_DOCKER_LOG}" || fail "缺少本地镜像检查"
if grep -Eq 'docker pull cached.registry/miniflux:minicache' "${MOCK_DOCKER_LOG}"; then
  fail "本地已有镜像时不应继续 pull"
fi

log_info "install cached image smoke passed"
