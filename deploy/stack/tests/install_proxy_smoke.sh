#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../scripts/common.sh"

TMP_DIR="$(mktemp -d)"
WORK_DIR="${TMP_DIR}/work"
STACK_DIR_REL="proxy-stack"
MOCK_BIN="${TMP_DIR}/mock-bin"
MOCK_DOCKER_LOG="${TMP_DIR}/docker.log"
MOCK_SYSTEMCTL_LOG="${TMP_DIR}/systemctl.log"
DOCKER_SYSTEMD_DROPIN_DIR="${TMP_DIR}/docker.service.d"
trap 'rm -rf "${TMP_DIR}"' EXIT

mkdir -p "${MOCK_BIN}" "${WORK_DIR}"

cat > "${MOCK_BIN}/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
log_file="${MOCK_DOCKER_LOG:?}"
printf 'docker %s\n' "$*" >> "${log_file}"
if [[ "${1:-}" == "image" && "${2:-}" == "inspect" ]]; then
  exit 1
fi
exit 0
EOF
chmod +x "${MOCK_BIN}/docker"

cat > "${MOCK_BIN}/systemctl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
log_file="${MOCK_SYSTEMCTL_LOG:?}"
printf 'systemctl %s\n' "$*" >> "${log_file}"
exit 0
EOF
chmod +x "${MOCK_BIN}/systemctl"

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
echo "mock-openssl" >&2
exit 1
EOF
chmod +x "${MOCK_BIN}/openssl"

export PATH="${MOCK_BIN}:${PATH}"
export MOCK_DOCKER_LOG
export MOCK_SYSTEMCTL_LOG
export DOCKER_SYSTEMD_DROPIN_DIR
export http_proxy="http://127.0.0.1:7890"
export https_proxy="http://127.0.0.1:7890"
export HTTP_PROXY="http://127.0.0.1:7890"
export HTTPS_PROXY="http://127.0.0.1:7890"
export NO_PROXY="corp.internal"

# shellcheck source=/dev/null
source "${SCRIPT_DIR}/../install.sh"

ensure_linux() { :; }
wait_for_http_ok() { return 0; }
bootstrap_miniflux() { printf 'mock-miniflux-token\n'; }
bootstrap_halo() { printf 'mock-halo-token\n'; }

(
  cd "${WORK_DIR}"
  main --profile full --stack-dir "${STACK_DIR_REL}" --force
)

dropin_file="${DOCKER_SYSTEMD_DROPIN_DIR}/http-proxy.conf"
[[ -f "${dropin_file}" ]] || fail "未生成 Docker daemon 代理 drop-in"
grep -q 'HTTP_PROXY=http://127.0.0.1:7890' "${dropin_file}" || fail "drop-in 缺少 HTTP_PROXY"
grep -q 'HTTPS_PROXY=http://127.0.0.1:7890' "${dropin_file}" || fail "drop-in 缺少 HTTPS_PROXY"
grep -q 'NO_PROXY=corp.internal,localhost,127.0.0.1,::1,postgres,redis,miniflux,halo' "${dropin_file}" || fail "drop-in 缺少合并后的 NO_PROXY"
grep -Eq 'systemctl daemon-reload' "${MOCK_SYSTEMCTL_LOG}" || fail "缺少 daemon-reload"
grep -Eq 'systemctl restart docker' "${MOCK_SYSTEMCTL_LOG}" || fail "缺少 docker restart"
grep -Eq 'docker pull postgres:17' "${MOCK_DOCKER_LOG}" || fail "缺少 postgres 镜像预拉取"
grep -Eq 'docker pull redis:7' "${MOCK_DOCKER_LOG}" || fail "缺少 redis 镜像预拉取"
grep -Eq 'docker pull ghcr.io/miniflux/miniflux:2.2.15' "${MOCK_DOCKER_LOG}" || fail "缺少 miniflux 镜像预拉取"
grep -Eq 'docker pull registry.fit2cloud.com/halo/halo:2.23' "${MOCK_DOCKER_LOG}" || fail "缺少 halo 镜像预拉取"

rendered_env="${WORK_DIR}/${STACK_DIR_REL}/.env"
[[ -f "${rendered_env}" ]] || fail "未生成 stack .env"
grep -q 'HTTP_PROXY=http://host.docker.internal:7890' "${rendered_env}" || fail ".env 运行时 HTTP_PROXY 未转换为 host.docker.internal"
grep -q 'HTTPS_PROXY=http://host.docker.internal:7890' "${rendered_env}" || fail ".env 运行时 HTTPS_PROXY 未转换为 host.docker.internal"
grep -q 'http_proxy=http://host.docker.internal:7890' "${rendered_env}" || fail ".env 运行时 http_proxy 未转换为 host.docker.internal"
grep -q 'https_proxy=http://host.docker.internal:7890' "${rendered_env}" || fail ".env 运行时 https_proxy 未转换为 host.docker.internal"
grep -q 'BUILD_HTTP_PROXY=http://127.0.0.1:7890' "${rendered_env}" || fail ".env 缺少原始构建 BUILD_HTTP_PROXY"
grep -q 'BUILD_HTTPS_PROXY=http://127.0.0.1:7890' "${rendered_env}" || fail ".env 缺少原始构建 BUILD_HTTPS_PROXY"
grep -q 'BUILD_NO_PROXY=corp.internal,localhost,127.0.0.1,::1,postgres,redis,miniflux,halo' "${rendered_env}" || fail ".env 缺少原始构建 BUILD_NO_PROXY"
grep -q 'NO_PROXY=corp.internal,localhost,127.0.0.1,::1,postgres,redis,miniflux,halo' "${rendered_env}" || fail ".env 缺少合并后的 NO_PROXY"
grep -q 'no_proxy=corp.internal,localhost,127.0.0.1,::1,postgres,redis,miniflux,halo' "${rendered_env}" || fail ".env 缺少合并后的 no_proxy"

log_info "install proxy smoke passed"
