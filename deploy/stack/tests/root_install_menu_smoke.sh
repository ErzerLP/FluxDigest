#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
TMP_DIR="$(mktemp -d)"
RESPONSES_FILE="${TMP_DIR}/responses.txt"
STACK_LOG="${TMP_DIR}/stack.log"
trap 'rm -rf "${TMP_DIR}"' EXIT

cat > "${RESPONSES_FILE}" <<'EOF'
quick
full
/opt/fluxdigest-stack
192.168.50.10
yes
yes
EOF

cat > "${TMP_DIR}/mock-whiptail" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
response_file="${MOCK_WHIPTAIL_RESPONSES:?}"
response="$(head -n 1 "${response_file}")"
tail -n +2 "${response_file}" > "${response_file}.tmp"
mv "${response_file}.tmp" "${response_file}"
printf '%s' "${response}" >&2
exit 0
EOF
chmod +x "${TMP_DIR}/mock-whiptail"

cat > "${TMP_DIR}/mock-stack-install" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "${STACK_LOG:?}"
exit 0
EOF
chmod +x "${TMP_DIR}/mock-stack-install"

export MOCK_WHIPTAIL_RESPONSES="${RESPONSES_FILE}"
export STACK_LOG
export WHIPTAIL_BIN="${TMP_DIR}/mock-whiptail"
export FLUXDIGEST_STACK_INSTALL_BIN="${TMP_DIR}/mock-stack-install"

bash "${ROOT}/install.sh"

grep -q -- '--action install --profile full --stack-dir /opt/fluxdigest-stack --host 192.168.50.10 --force' "${STACK_LOG}" || {
  echo "interactive installer did not dispatch expected install args" >&2
  exit 1
}

