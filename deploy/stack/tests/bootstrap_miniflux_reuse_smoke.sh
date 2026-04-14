#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
TMP_DIR="$(mktemp -d)"
PORT=39191
trap '[[ -n "${SERVER_PID:-}" ]] && kill "${SERVER_PID}" 2>/dev/null || true; rm -rf "${TMP_DIR}"' EXIT

python_cmd=""
if command -v python >/dev/null 2>&1 && python --version >/dev/null 2>&1; then
  python_cmd="python"
elif command -v python3 >/dev/null 2>&1 && python3 --version >/dev/null 2>&1; then
  python_cmd="python3"
else
  echo "python/python3 is required for bootstrap_miniflux_reuse_smoke.sh" >&2
  exit 1
fi

cat > "${TMP_DIR}/server.py" <<'PY'
import json
from http.server import BaseHTTPRequestHandler, HTTPServer


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        if self.path == "/v1/api-keys":
            self.send_error(500, "should not create api key")
            return
        self.send_error(404)

    def do_GET(self):
        if self.path == "/v1/me" and self.headers.get("X-Auth-Token") == "existing-miniflux-token":
            body = json.dumps({"username": "admin"}).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        self.send_error(401)

    def log_message(self, *_args):
        return


HTTPServer(("127.0.0.1", 39191), Handler).serve_forever()
PY

"${python_cmd}" "${TMP_DIR}/server.py" &
SERVER_PID=$!
sleep 0.5

# shellcheck source=/dev/null
source "${ROOT_DIR}/deploy/stack/scripts/bootstrap_miniflux.sh"

export MINIFLUX_BASE_URL="http://127.0.0.1:${PORT}"
export MINIFLUX_ADMIN_USERNAME="admin"
export MINIFLUX_ADMIN_PASSWORD="secret"
export MINIFLUX_API_KEY_DESCRIPTION="FluxDigest Installer"
export APP_MINIFLUX_AUTH_TOKEN="existing-miniflux-token"

token="$(bootstrap_miniflux)"
[[ "${token}" == "existing-miniflux-token" ]]

echo "bootstrap_miniflux reuse smoke passed"
