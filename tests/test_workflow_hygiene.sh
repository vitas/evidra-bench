#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

module_go_version="$(awk '$1 == "go" { print $2; exit }' go.mod)"
docker_go_version="$(sed -n 's/^FROM golang:\([0-9][0-9.]*\)-alpine AS bench-builder$/\1/p' Dockerfile.bench)"
[[ -n "$docker_go_version" ]] \
  || fail "Dockerfile.bench should declare the bench-builder Go version"
[[ "$docker_go_version" == "$module_go_version" ]] \
  || fail "Dockerfile.bench Go $docker_go_version should match go.mod Go $module_go_version"

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

grep -q -- 'make vuln' .github/workflows/ci.yml \
  || fail "CI should run govulncheck"
grep -q -- 'make vuln' .github/workflows/release.yml \
  || fail "release should run govulncheck"

grep -q -- 'bash tests/test_artifact_hygiene.sh' .github/workflows/ci.yml \
  || fail "CI should run artifact hygiene checks"
grep -q -- 'bash tests/test_artifact_hygiene.sh' .github/workflows/release.yml \
  || fail "release should run artifact hygiene checks"

grep -q -- 'bash tests/test_secret_hygiene.sh' .github/workflows/ci.yml \
  || fail "CI should run repository secret hygiene checks"
grep -q -- 'bash tests/test_secret_hygiene.sh' .github/workflows/release.yml \
  || fail "release should run repository secret hygiene checks"

grep -q -- 'bash tests/test_ui_secret_hygiene.sh' .github/workflows/ci.yml \
  || fail "CI should run UI secret hygiene checks"
grep -q -- 'bash tests/test_ui_secret_hygiene.sh' .github/workflows/release.yml \
  || fail "release should run UI secret hygiene checks"

grep -q -- 'bash tests/smoke/test_private_review_smoke.sh' .github/workflows/ci.yml \
  || fail "CI should run private review smoke script self-test"
grep -q -- 'bash tests/smoke/test_private_review_smoke.sh' .github/workflows/release.yml \
  || fail "release should run private review smoke script self-test"

grep -q -- 'bash tests/test_dco_signoff.sh' .github/workflows/ci.yml \
  || fail "CI should run DCO sign-off checks"
grep -q -- 'bash tests/test_dco_signoff.sh' .github/workflows/release.yml \
  || fail "release should run DCO sign-off checks"

grep -Fq -- 'node --test src/lib/*.test.mts' .github/workflows/ci.yml \
  || fail "CI should run all UI lib tests"
grep -Fq -- 'node --test src/lib/*.test.mts' .github/workflows/release.yml \
  || fail "release should run all UI lib tests"

if grep -R -nE --include='*.sh' 'git[[:space:]]+grep' tests | grep -v 'tests/test_workflow_hygiene.sh'; then
  fail "shell tests should use portable grep/rg checks instead of direct git-grep"
fi

awk '/^  release:/{in_release=1; next} /^  docker:/{in_release=0} in_release {print}' .github/workflows/release.yml |
  grep -q -- "if: startsWith(github.ref, 'refs/tags/')" \
  || fail "release job should run only for tag refs"

echo "PASS: test_workflow_hygiene"
