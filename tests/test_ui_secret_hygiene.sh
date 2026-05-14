#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

if grep -R -nE 'VITE_BENCH_API_KEY|secrets\.BENCH_API_KEY' ui .github/workflows .env.example docs/guides/bench-service-setup.md; then
  fail "public UI build must not receive or reference BENCH_API_KEY"
fi

if grep -R -nE 'Authorization' ui/src; then
  fail "browser UI must not send static Authorization headers"
fi

echo "PASS: test_ui_secret_hygiene"
