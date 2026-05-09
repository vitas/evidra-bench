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


class Handler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        return

    def write_json(self, status, body):
        payload = json.dumps(body).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def do_GET(self):
        path = self.path.split("?", 1)[0]
        if path == "/healthz":
            self.write_json(200, {"status": "ok"})
            return
        if path == "/v1/bench/scenarios":
            self.write_json(200, {"scenarios": [{"id": "s1", "category": "kubernetes"}]})
            return
        if path == "/v1/bench/leaderboard":
            self.write_json(200, {"models": []})
            return
        if path == "/v1/bench/runs":
            self.write_json(200, {"runs": [], "total": 0, "limit": 5, "offset": 0})
            return
        self.write_json(404, {"error": "not found"})

    def do_POST(self):
        if self.path.split("?", 1)[0] == "/v1/bench/runs":
            self.write_json(401, {"error": "unauthorized"})
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
    bash "$ROOT_DIR/tests/smoke/run_public_api_smoke.sh"
)"

if ! grep -q "All public API smoke checks passed" <<<"$OUTPUT"; then
  echo "$OUTPUT" >&2
  echo "missing success line" >&2
  exit 1
fi
