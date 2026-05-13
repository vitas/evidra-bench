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

if git grep -nI -E "$combined" -- \
  . \
  ':!go.sum' \
  ':!ui/package-lock.json' \
  ':!docs/archive/**'; then
  echo "FAIL: possible secret pattern found in tracked files" >&2
  exit 1
fi

echo "PASS: test_secret_hygiene"
