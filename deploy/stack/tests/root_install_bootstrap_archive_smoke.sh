#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
TMP_DIR="$(mktemp -d)"
RUNNER_DIR="${TMP_DIR}/runner"
ARCHIVE_ROOT="${TMP_DIR}/archive-root"
MOCK_BIN="${TMP_DIR}/mock-bin"
STACK_LOG="${TMP_DIR}/stack.log"
CURL_LOG="${TMP_DIR}/curl.log"
CALL_COUNT_FILE="${TMP_DIR}/curl.count"
ARCHIVE_PATH="${TMP_DIR}/fluxdigest-bootstrap.tar.gz"
trap 'rm -rf "${TMP_DIR}"' EXIT

mkdir -p "${RUNNER_DIR}" "${ARCHIVE_ROOT}/FluxDigest-master/deploy/stack" "${MOCK_BIN}"
cp "${ROOT}/install.sh" "${RUNNER_DIR}/install.sh"

cat > "${ARCHIVE_ROOT}/FluxDigest-master/deploy/stack/install.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "${STACK_LOG:?}"
EOF
chmod +x "${ARCHIVE_ROOT}/FluxDigest-master/deploy/stack/install.sh"

tar -C "${ARCHIVE_ROOT}" -czf "${ARCHIVE_PATH}" "FluxDigest-master"

cat > "${MOCK_BIN}/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

log_file="${CURL_LOG:?}"
count_file="${CALL_COUNT_FILE:?}"
archive_path="${ARCHIVE_PATH:?}"

output_file=""
args=("$@")
for ((i = 0; i < ${#args[@]}; i++)); do
  case "${args[$i]}" in
    -o)
      ((i += 1))
      output_file="${args[$i]}"
      ;;
  esac
done

url="${args[$((${#args[@]} - 1))]}"
count=0
if [[ -f "${count_file}" ]]; then
  count="$(cat "${count_file}")"
fi
count=$((count + 1))
printf '%s' "${count}" > "${count_file}"

printf 'call=%s args=%s\n' "${count}" "$*" >> "${log_file}"

if [[ "${count}" -eq 1 ]]; then
  [[ "${url}" == "https://github.com/ErzerLP/FluxDigest/archive/refs/heads/master.tar.gz" ]] || {
    echo "unexpected first bootstrap url: ${url}" >&2
    exit 1
  }
  exit 22
fi

[[ "${url}" == "https://ghproxy.vip/https://github.com/ErzerLP/FluxDigest/archive/refs/heads/master.tar.gz" ]] || {
  echo "unexpected second bootstrap url: ${url}" >&2
  exit 1
}

[[ " $* " == *" --noproxy ghproxy.vip "* ]] || {
  echo "expected --noproxy ghproxy.vip for mirror bootstrap" >&2
  exit 1
}

[[ -n "${output_file}" ]] || {
  echo "missing -o output file" >&2
  exit 1
}

cp "${archive_path}" "${output_file}"
EOF
chmod +x "${MOCK_BIN}/curl"

export PATH="${MOCK_BIN}:${PATH}"
export STACK_LOG CURL_LOG CALL_COUNT_FILE ARCHIVE_PATH
export FLUXDIGEST_BOOTSTRAP_ARCHIVE_URLS="https://github.com/ErzerLP/FluxDigest/archive/refs/heads/master.tar.gz https://ghproxy.vip/https://github.com/ErzerLP/FluxDigest/archive/refs/heads/master.tar.gz"

bash "${RUNNER_DIR}/install.sh" --action status --stack-dir /tmp/fluxdigest-stack

grep -q -- '--action status --stack-dir /tmp/fluxdigest-stack' "${STACK_LOG}" || {
  echo "root installer did not dispatch expected args after bootstrap" >&2
  exit 1
}

grep -q -- 'call=1 args=' "${CURL_LOG}" || {
  echo "missing first bootstrap download attempt" >&2
  exit 1
}

grep -q -- 'call=2 args=' "${CURL_LOG}" || {
  echo "missing second bootstrap download attempt" >&2
  exit 1
}
