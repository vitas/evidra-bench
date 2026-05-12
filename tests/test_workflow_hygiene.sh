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

grep -q -- 'bash tests/test_artifact_hygiene.sh' .github/workflows/ci.yml \
  || fail "CI should run artifact hygiene checks"
grep -q -- 'bash tests/test_artifact_hygiene.sh' .github/workflows/release.yml \
  || fail "release should run artifact hygiene checks"

grep -q -- 'bash tests/test_ui_secret_hygiene.sh' .github/workflows/ci.yml \
  || fail "CI should run UI secret hygiene checks"
grep -q -- 'bash tests/test_ui_secret_hygiene.sh' .github/workflows/release.yml \
  || fail "release should run UI secret hygiene checks"

awk '/^  release:/{in_release=1; next} /^  docker:/{in_release=0} in_release {print}' .github/workflows/release.yml |
  grep -q -- "if: startsWith(github.ref, 'refs/tags/')" \
  || fail "release job should run only for tag refs"

echo "PASS: test_workflow_hygiene"
