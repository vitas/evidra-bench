#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

fail_found() {
  local description="$1"
  shift
  if "$@"; then
    echo "legacy core evidra dependency found: ${description}" >&2
    return 1
  fi
  return 0
}

fail_found "go.mod require/replace samebits.com/evidra" \
  grep -nE 'samebits\.com/evidra(\s|$)' go.mod

fail_found "Go imports from core evidra" \
  sh -c "git grep -n '\"samebits.com/evidra/' -- '*.go'"

fail_found "CI/release checkout of core evidra" \
  grep -rnE 'Checkout evidra|repository: .*/evidra$' .github/workflows

fail_found "Dockerfile.bench cloning/building core evidra" \
  grep -nE 'EVIDRA_REPO|EVIDRA_REF|/build/parent|cmd/evidra-mcp' Dockerfile.bench

fail_found "bench CLI special evidra mode flag registrations" \
  sh -c "git grep -nE '(StringVar|BoolVar|StringSliceVar|StringP|BoolP)[^(]*\\([^)]*\"(evidra-bin|evidra-evidence-dir|proxy-mode|smart-prescribe|trace|evidra)\"' -- cmd pkg internal ':(exclude)*_test.go'"
