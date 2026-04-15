#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
TMP_DIR="$(mktemp -d)"
PORT=39312
trap '[[ -n "${SERVER_PID:-}" ]] && kill "${SERVER_PID}" 2>/dev/null || true; rm -rf "${TMP_DIR}"' EXIT

python_cmd=""
if command -v python >/dev/null 2>&1 && python --version >/dev/null 2>&1; then
  python_cmd="python"
elif command -v python3 >/dev/null 2>&1 && python3 --version >/dev/null 2>&1; then
  python_cmd="python3"
else
  echo "python/python3 is required for bootstrap_halo_basic_fallback_smoke.sh" >&2
  exit 1
fi

cat > "${TMP_DIR}/server.py" <<'PY'
import base64
import json
from http.server import BaseHTTPRequestHandler, HTTPServer


basic_value = "Basic " + base64.b64encode(b"halo-admin:halo-secret").decode()


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        if self.path == "/apis/uc.api.security.halo.run/v1alpha1/personalaccesstokens":
            body = json.dumps({
                "metadata": {
                    "name": "fluxdigest-pat",
                    "annotations": {
                        "security.halo.run/access-token": "pat_generated_access_token",
                    },
                },
            }).encode()
            self.send_response(201)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        self.send_error(404)

    def do_GET(self):
        if self.path.startswith("/apis/api.console.halo.run/v1alpha1/posts"):
            if self.headers.get("Authorization") == "Bearer pat_generated_access_token":
                self.send_response(403)
                self.end_headers()
                return
            if self.headers.get("Authorization") == basic_value:
                body = json.dumps({"items": []}).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)
                return
        self.send_error(401)

    def log_message(self, *_args):
        return


HTTPServer(("127.0.0.1", 39312), Handler).serve_forever()
PY

"${python_cmd}" "${TMP_DIR}/server.py" &
SERVER_PID=$!
sleep 0.5

# shellcheck source=/dev/null
source "${ROOT_DIR}/deploy/stack/scripts/bootstrap_halo.sh"

export HALO_BASE_URL="http://127.0.0.1:${PORT}"
export HALO_ADMIN_USERNAME="halo-admin"
export HALO_ADMIN_PASSWORD="halo-secret"
export HALO_PAT_NAME="FluxDigest Publisher"
export APP_PUBLISH_HALO_TOKEN=""

token="$(bootstrap_halo)"
[[ "${token}" == "basic:aGFsby1hZG1pbjpoYWxvLXNlY3JldA==" ]]

echo "bootstrap_halo basic fallback smoke passed"
