#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
TMP_DIR="$(mktemp -d)"
STACK_LOG="${TMP_DIR}/stack.log"
trap 'rm -rf "${TMP_DIR}"' EXIT

cat > "${TMP_DIR}/mock-stack-install.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "${STACK_LOG:?}"
EOF
chmod 644 "${TMP_DIR}/mock-stack-install.sh"

export STACK_LOG
export FLUXDIGEST_STACK_INSTALL_BIN="${TMP_DIR}/mock-stack-install.sh"

bash "${ROOT}/install.sh" --action status --stack-dir /tmp/fluxdigest-stack

grep -q -- '--action status --stack-dir /tmp/fluxdigest-stack' "${STACK_LOG}" || {
  echo "root installer did not dispatch expected args to non-executable stack bin" >&2
  exit 1
}
