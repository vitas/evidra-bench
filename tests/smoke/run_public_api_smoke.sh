#!/usr/bin/env bash
set -euo pipefail

API_URL="${1:-${BENCH_API_URL:-}}"

if [[ -z "$API_URL" ]]; then
  echo "usage: BENCH_API_URL=https://api.example.com bash tests/smoke/run_public_api_smoke.sh" >&2
  echo "   or: bash tests/smoke/run_public_api_smoke.sh https://api.example.com" >&2
  exit 2
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "missing dependency: curl" >&2
  exit 2
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "missing dependency: python3" >&2
  exit 2
fi

API_URL="${API_URL%/}"
TMP_DIR="$(mktemp -d)"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

fail() {
  local message="$1"
  local body="${2:-}"
  echo "FAIL: ${message}" >&2
  if [[ -n "$body" && -s "$body" ]]; then
    echo "--- response body ---" >&2
    sed -n '1,40p' "$body" >&2
  fi
  exit 1
}

json_has_array_key() {
  local body="$1"
  local keys="$2"
  python3 - "$body" "$keys" <<'PY'
import json
import sys

path, keys = sys.argv[1], sys.argv[2].split(",")
with open(path, "r", encoding="utf-8") as f:
    data = json.load(f)

if not isinstance(data, dict):
    raise SystemExit(1)

if not any(isinstance(data.get(key), list) for key in keys):
    raise SystemExit(1)
PY
}

json_health_ok() {
  local body="$1"
  python3 - "$body" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as f:
    data = json.load(f)

if not isinstance(data, dict) or data.get("status") != "ok":
    raise SystemExit(1)
PY
}

check_get() {
  local label="$1"
  local path="$2"
  local shape="$3"
  local body="$TMP_DIR/${label}.json"
  local status

  status="$(curl -sS -o "$body" -w "%{http_code}" "$API_URL$path")" ||
    fail "${label}: request failed" "$body"

  if [[ "$status" != "200" ]]; then
    fail "${label}: expected HTTP 200, got ${status}" "$body"
  fi

  case "$shape" in
    health)
      json_health_ok "$body" || fail "${label}: expected JSON {status: ok}" "$body"
      ;;
    array:*)
      json_has_array_key "$body" "${shape#array:}" ||
        fail "${label}: expected JSON array key ${shape#array:}" "$body"
      ;;
    *)
      fail "${label}: unknown response shape ${shape}"
      ;;
  esac

  echo "PASS: ${label}"
}

check_unauthenticated_write_rejected() {
  local body="$TMP_DIR/write-reject.json"
  local status

  status="$(
    curl -sS -o "$body" -w "%{http_code}" \
      -X POST \
      -H "Content-Type: application/json" \
      -d '{}' \
      "$API_URL/v1/bench/runs"
  )" || fail "write auth check: request failed" "$body"

  if [[ "$status" != "401" ]]; then
    fail "write auth check: expected HTTP 401, got ${status}" "$body"
  fi

  echo "PASS: unauthenticated POST /v1/bench/runs is rejected"
}

echo "=== Bench public API smoke ==="
echo "API: ${API_URL}"

check_get "healthz" "/healthz" "health"
check_get "scenarios" "/v1/bench/scenarios" "array:scenarios,items"
check_get "leaderboard" "/v1/bench/leaderboard" "array:models"
check_get "runs" "/v1/bench/runs?limit=5" "array:runs"
check_get "runs-scenario-list-filter" "/v1/bench/runs?scenarios=__smoke_a,__smoke_b&limit=5" "array:runs"
check_unauthenticated_write_rejected

echo "=== All public API smoke checks passed ==="
