#!/usr/bin/env bash
set -euo pipefail

API_URL="${1:-${BENCH_API_URL:-}}"
API_KEY="${BENCH_API_KEY:-}"
RUN_ID="${BENCH_REVIEW_SMOKE_RUN_ID:-}"

if [[ -z "$API_URL" || -z "$API_KEY" || -z "$RUN_ID" ]]; then
  echo "usage: BENCH_API_URL=https://bench.example BENCH_API_KEY=... BENCH_REVIEW_SMOKE_RUN_ID=... bash tests/smoke/run_private_review_smoke.sh" >&2
  echo "   or: BENCH_API_KEY=... BENCH_REVIEW_SMOKE_RUN_ID=... bash tests/smoke/run_private_review_smoke.sh https://bench.example" >&2
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
COOKIE_JAR="$TMP_DIR/cookies.txt"
ANON_COOKIE_JAR="$TMP_DIR/anonymous-cookies.txt"
LOGIN_PAYLOAD="$TMP_DIR/login.json"
REVIEW_PAYLOAD="$TMP_DIR/review.json"

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

write_payloads() {
  python3 - "$LOGIN_PAYLOAD" "$REVIEW_PAYLOAD" "$API_KEY" "$RUN_ID" <<'PY'
import json
import sys

login_path, review_path, api_key, run_id = sys.argv[1:5]

with open(login_path, "w", encoding="utf-8") as f:
    json.dump({"api_key": api_key}, f)

review = {
    "version": "run_review.v1",
    "run_id": run_id,
    "visibility": "private",
    "verdict": "unsafe_pass",
    "primary_label": "unsafe_action",
    "reviewer": {"id": "private-review-smoke", "display": "Private Review Smoke"},
    "labels": [
        {
            "kind": "unsafe_action",
            "severity": "warning",
            "step": 1,
            "note": "Private deployment review smoke marker.",
            "evidence_snippet": "private-review-smoke evidence",
        }
    ],
}
with open(review_path, "w", encoding="utf-8") as f:
    json.dump(review, f)
PY
}

json_expect_auth() {
  local body="$1"
  local want="$2"
  python3 - "$body" "$want" <<'PY'
import json
import sys

path, want_raw = sys.argv[1:3]
want = want_raw == "true"
with open(path, "r", encoding="utf-8") as f:
    data = json.load(f)

if not isinstance(data, dict) or data.get("authenticated") is not want:
    raise SystemExit(1)
PY
}

json_expect_private_review() {
  local body="$1"
  python3 - "$body" "$RUN_ID" <<'PY'
import json
import sys

path, run_id = sys.argv[1:3]
with open(path, "r", encoding="utf-8") as f:
    data = json.load(f)

if not isinstance(data, dict):
    raise SystemExit(1)
if data.get("run_id") != run_id:
    raise SystemExit(1)
if data.get("visibility") != "private" or data.get("verdict") != "unsafe_pass":
    raise SystemExit(1)
PY
}

request_with_session() {
  local label="$1"
  local body="$2"
  shift 2
  local status

  status="$(curl -sS -b "$COOKIE_JAR" -c "$COOKIE_JAR" -o "$body" -w "%{http_code}" "$@")" ||
    fail "${label}: request failed" "$body"
  printf '%s' "$status"
}

request_anonymous() {
  local label="$1"
  local body="$2"
  shift 2
  local status

  status="$(curl -sS -b "$ANON_COOKIE_JAR" -c "$ANON_COOKIE_JAR" -o "$body" -w "%{http_code}" "$@")" ||
    fail "${label}: request failed" "$body"
  printf '%s' "$status"
}

expect_status() {
  local label="$1"
  local got="$2"
  local want="$3"
  local body="$4"

  if [[ "$got" != "$want" ]]; then
    fail "${label}: expected HTTP ${want}, got ${got}" "$body"
  fi
  echo "PASS: ${label}"
}

write_payloads

echo "=== Bench private review smoke ==="
echo "API: ${API_URL}"
echo "Run: ${RUN_ID}"

body="$TMP_DIR/session-anonymous.json"
status="$(request_anonymous "anonymous session status" "$body" "$API_URL/v1/bench/session")"
expect_status "anonymous session status" "$status" "200" "$body"
json_expect_auth "$body" false || fail "anonymous session status: expected authenticated false" "$body"

body="$TMP_DIR/unauthenticated-review-put.json"
status="$(
  request_anonymous "unauthenticated review write rejection" "$body" \
    -X PUT \
    -H "Content-Type: application/json" \
    --data-binary "@$REVIEW_PAYLOAD" \
    "$API_URL/v1/bench/runs/$RUN_ID/review"
)"
expect_status "unauthenticated review write rejection" "$status" "401" "$body"

body="$TMP_DIR/session-login.json"
status="$(
  request_with_session "session login" "$body" \
    -X POST \
    -H "Content-Type: application/json" \
    --data-binary "@$LOGIN_PAYLOAD" \
    "$API_URL/v1/bench/session"
)"
expect_status "session login" "$status" "200" "$body"
json_expect_auth "$body" true || fail "session login: expected authenticated true" "$body"
grep -q 'bench_session' "$COOKIE_JAR" || fail "session login: bench_session cookie missing"

body="$TMP_DIR/session-authenticated.json"
status="$(request_with_session "authenticated session status" "$body" "$API_URL/v1/bench/session")"
expect_status "authenticated session status" "$status" "200" "$body"
json_expect_auth "$body" true || fail "authenticated session status: expected authenticated true" "$body"

body="$TMP_DIR/review-put.json"
status="$(
  request_with_session "session review write" "$body" \
    -X PUT \
    -H "Content-Type: application/json" \
    --data-binary "@$REVIEW_PAYLOAD" \
    "$API_URL/v1/bench/runs/$RUN_ID/review"
)"
expect_status "session review write" "$status" "200" "$body"
json_expect_private_review "$body" || fail "session review write: expected private unsafe_pass review" "$body"

body="$TMP_DIR/review-get-authenticated.json"
status="$(request_with_session "authenticated private review read" "$body" "$API_URL/v1/bench/runs/$RUN_ID/review")"
expect_status "authenticated private review read" "$status" "200" "$body"
json_expect_private_review "$body" || fail "authenticated private review read: expected private unsafe_pass review" "$body"

body="$TMP_DIR/session-logout.json"
status="$(request_with_session "session logout" "$body" -X DELETE "$API_URL/v1/bench/session")"
expect_status "session logout" "$status" "204" "$body"

body="$TMP_DIR/session-after-logout.json"
status="$(request_with_session "session status after logout" "$body" "$API_URL/v1/bench/session")"
expect_status "session status after logout" "$status" "200" "$body"
json_expect_auth "$body" false || fail "session status after logout: expected authenticated false" "$body"

body="$TMP_DIR/review-get-anonymous.json"
status="$(request_anonymous "anonymous private review hidden" "$body" "$API_URL/v1/bench/runs/$RUN_ID/review")"
expect_status "anonymous private review hidden" "$status" "404" "$body"

echo "=== All private review smoke checks passed ==="
