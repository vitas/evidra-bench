#!/usr/bin/env bash
set -euo pipefail

target="${1:-../evidra-bench-public}"

if [[ -e "$target" ]]; then
  echo "FAIL: target already exists: $target" >&2
  exit 1
fi

mkdir -p "$target"
git archive --format=tar HEAD | tar -x -C "$target"

(
  cd "$target"
  git init -b main
  git add -f .
  git commit -s -m "chore: import evidra bench open source baseline"
)

echo "Public export created at: $target"
echo "Review it before pushing or changing repository visibility."
