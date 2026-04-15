#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

STACK_TEMPLATE_DIR="${STACK_TEMPLATE_DIR:-${SCRIPT_DIR}/..}"
STACK_DIR="${STACK_DIR:-${STACK_TEMPLATE_DIR}/rendered-stack}"

render_template() {
  local template_file="${1:?}"
  local dest_file="${2:?}"

  if [[ ! -f "${template_file}" ]]; then
    fail "找不到模板: ${template_file}"
  fi

  local interpreter=""
  if command -v python >/dev/null 2>&1; then
    interpreter=python
  elif command -v python3 >/dev/null 2>&1; then
    interpreter=python3
  else
    fail "渲染模板需要 python3/python"
  fi

  mkdir -p "$(dirname "${dest_file}")"
  local python_script
  python_script="$(cat <<'PY'
import os
import re
import sys
from pathlib import Path

src = Path(sys.argv[1])
dst = Path(sys.argv[2])
text = src.read_text(encoding="utf-8")
pattern = re.compile(r"\{\{\s*([A-Za-z0-9_]+)\s*\}\}")
def repl(match):
    return os.environ.get(match.group(1), "")
text = pattern.sub(repl, text)
text = text.replace("__MINIFLUX_SERVICE_BLOCK__", os.environ.get("STACK_MINIFLUX_SERVICE_BLOCK", ""))
text = text.replace("__HALO_SERVICE_BLOCK__", os.environ.get("STACK_HALO_SERVICE_BLOCK", ""))
dst.write_text(text, encoding="utf-8")
PY
)"
  "${interpreter}" -c "${python_script}" "${template_file}" "${dest_file}"
}

render_stack_files() {
  mkdir -p "${STACK_DIR}/initdb"

  local env_template="${STACK_TEMPLATE_DIR}/stack.env.tpl"
  local compose_template="${STACK_TEMPLATE_DIR}/docker-compose.yml.tpl"
  local sql_template="${STACK_TEMPLATE_DIR}/initdb/01-init-app-dbs.sql.tpl"

  render_template "${env_template}" "${STACK_DIR}/.env"
  render_template "${compose_template}" "${STACK_DIR}/docker-compose.yml"
  render_template "${sql_template}" "${STACK_DIR}/initdb/01-init-app-dbs.sql"

  log_info "Rendered stack files into ${STACK_DIR}"
}
