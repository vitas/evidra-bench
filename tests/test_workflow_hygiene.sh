#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

for workflow in .github/workflows/*.yml; do
  if grep -q -- 'go-version-file: bench/go.mod' "$workflow" &&
    ! grep -q -- 'cache-dependency-path: bench/go.sum' "$workflow"; then
    fail "$workflow should set setup-go cache-dependency-path when checkout uses path: bench"
  fi
done

grep -q -- 'bash tests/test_workflow_hygiene.sh' .github/workflows/ci.yml \
  || fail "CI should run workflow hygiene checks"
grep -q -- 'bash tests/test_workflow_hygiene.sh' .github/workflows/release.yml \
  || fail "release should run workflow hygiene checks"

echo "PASS: test_workflow_hygiene"
