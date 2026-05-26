#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
TMP_DIR="$(mktemp -d)"
PORT_FILE="$TMP_DIR/port"

cleanup() {
  if [[ -n "${SERVER_PID:-}" ]]; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

python3 - "$PORT_FILE" <<'PY' &
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

port_file = sys.argv[1]
api_key = "secret-key"
session_cookie = "bench_session=fixture-session; Path=/; HttpOnly; SameSite=Lax"
review_saved = False


class Handler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        return

    def read_json(self):
        length = int(self.headers.get("Content-Length", "0"))
        if length == 0:
            return {}
        return json.loads(self.rfile.read(length))

    def write_json(self, status, body, headers=None):
        payload = json.dumps(body).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        for key, value in (headers or {}).items():
            self.send_header(key, value)
        self.end_headers()
        self.wfile.write(payload)

    def write_empty(self, status, headers=None):
        self.send_response(status)
        for key, value in (headers or {}).items():
            self.send_header(key, value)
        self.end_headers()

    def has_session(self):
        return "bench_session=fixture-session" in self.headers.get("Cookie", "")

    def do_GET(self):
        global review_saved
        path = self.path.split("?", 1)[0]
        if path == "/v1/bench/session":
            if self.has_session():
                self.write_json(200, {"authenticated": True, "tenant_id": "default"})
            else:
                self.write_json(200, {"authenticated": False})
            return
        if path == "/v1/bench/runs/smoke-run/review":
            if not self.has_session() or not review_saved:
                self.write_json(404, {"error": "not found"})
                return
            self.write_json(200, {
                "version": "run_review.v1",
                "run_id": "smoke-run",
                "visibility": "private",
                "verdict": "unsafe_pass",
                "labels": [{"kind": "unsafe_action", "severity": "warning", "note": "smoke"}],
            })
            return
        self.write_json(404, {"error": "not found"})

    def do_POST(self):
        if self.path.split("?", 1)[0] == "/v1/bench/session":
            body = self.read_json()
            if body.get("api_key") != api_key:
                self.write_json(401, {"error": "unauthorized"})
                return
            self.write_json(
                200,
                {"authenticated": True, "tenant_id": "default"},
                {"Set-Cookie": session_cookie},
            )
            return
        self.write_json(404, {"error": "not found"})

    def do_PUT(self):
        global review_saved
        if self.path.split("?", 1)[0] == "/v1/bench/runs/smoke-run/review":
            if not self.has_session():
                self.write_json(401, {"error": "unauthorized"})
                return
            body = self.read_json()
            if body.get("visibility") != "private" or body.get("verdict") != "unsafe_pass":
                self.write_json(400, {"error": "bad review"})
                return
            review_saved = True
            self.write_json(200, {
                "version": "run_review.v1",
                "run_id": "smoke-run",
                "visibility": "private",
                "verdict": "unsafe_pass",
            })
            return
        self.write_json(404, {"error": "not found"})

    def do_DELETE(self):
        if self.path.split("?", 1)[0] == "/v1/bench/session":
            self.write_empty(204, {"Set-Cookie": "bench_session=; Path=/; Max-Age=0; HttpOnly; SameSite=Lax"})
            return
        self.write_json(404, {"error": "not found"})


server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
with open(port_file, "w", encoding="utf-8") as f:
    f.write(str(server.server_port))
server.serve_forever()
PY
SERVER_PID=$!

for _ in {1..50}; do
  [[ -s "$PORT_FILE" ]] && break
  sleep 0.1
done

if [[ ! -s "$PORT_FILE" ]]; then
  echo "fake API did not start" >&2
  exit 1
fi

PORT="$(cat "$PORT_FILE")"
OUTPUT="$(
  BENCH_API_URL="http://127.0.0.1:${PORT}" \
  BENCH_API_KEY="secret-key" \
  BENCH_REVIEW_SMOKE_RUN_ID="smoke-run" \
    bash "$ROOT_DIR/tests/smoke/run_private_review_smoke.sh"
)"

if ! grep -q "All private review smoke checks passed" <<<"$OUTPUT"; then
  echo "$OUTPUT" >&2
  echo "missing success line" >&2
  exit 1
fi
