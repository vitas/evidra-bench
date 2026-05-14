#!/usr/bin/env bash
set -euo pipefail

patterns=(
  'AKIA[0-9A-Z]{16}'
  'ASIA[0-9A-Z]{16}'
  'ghp_[A-Za-z0-9_]{30,}'
  'github_pat_[A-Za-z0-9_]{40,}'
  'sk-[A-Za-z0-9]{20,}'
  'xox[baprs]-[A-Za-z0-9-]{20,}'
  'BEGIN (RSA|DSA|EC|OPENSSH|PRIVATE) PRIVATE KEY'
)

combined="$(IFS='|'; echo "${patterns[*]}")"

set +e
matches="$(rg --hidden -nI -e "$combined" \
  --glob '!.git/**' \
  --glob '!go.sum' \
  --glob '!ui/package-lock.json' \
  --glob '!docs/archive/**' \
  . 2>&1)"
status=$?
set -e

if [[ "$status" -eq 0 ]]; then
  echo "$matches" >&2
  echo "FAIL: possible secret pattern found in repository files" >&2
  exit 1
fi
if [[ "$status" -gt 1 ]]; then
  echo "$matches" >&2
  echo "FAIL: secret hygiene search failed" >&2
  exit 1
fi

echo "PASS: test_secret_hygiene"
