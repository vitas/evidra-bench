#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

if git ls-files --error-unmatch runs >/dev/null 2>&1; then
  echo "Tracked generated run artifacts:" >&2
  git ls-files runs >&2
  fail "runs/ must not be tracked"
fi

if git ls-files --error-unmatch docs/plans >/dev/null 2>&1; then
  echo "Tracked planning docs:" >&2
  git ls-files docs/plans >&2
  fail "docs/plans/ must not be tracked"
fi

grep -q '^runs/$' .gitignore ||
  fail ".gitignore should ignore generated runs/"
if grep -q '^!runs/' .gitignore; then
  fail ".gitignore should not unignore generated runs/ content"
fi

grep -q '^docs/plans/$' .gitignore ||
  fail ".gitignore should ignore docs/plans/"

grep -q '^runs/$' .dockerignore ||
  fail ".dockerignore should exclude generated runs/ from Docker build context"

echo "PASS: test_artifact_hygiene"
