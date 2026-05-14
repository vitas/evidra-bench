#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

fail_found() {
  local description="$1"
  shift
  local output status
  set +e
  output="$("$@" 2>&1)"
  status=$?
  set -e
  if [[ "$status" -eq 0 ]]; then
    echo "$output" >&2
    echo "legacy core evidra dependency found: ${description}" >&2
    return 1
  fi
  if [[ "$status" -gt 1 ]]; then
    echo "$output" >&2
    echo "legacy dependency search failed: ${description}" >&2
    return 1
  fi
  return 0
}

rg_repo() {
  rg --hidden --glob '!.git/**' "$@"
}

fail_found "go.mod require/replace samebits.com/evidra" \
  rg_repo -n -e 'samebits\.com/evidra(\s|$)' go.mod

fail_found "Go imports from core evidra" \
  rg_repo -n -e '"samebits.com/evidra/' --glob '*.go' .

fail_found "CI/release checkout of core evidra" \
  rg_repo -n -e 'Checkout evidra|repository: .*/evidra$' .github/workflows

fail_found "Dockerfile.bench cloning/building core evidra" \
  rg_repo -n -e 'EVIDRA_REPO|EVIDRA_REF|/build/parent|cmd/evidra-mcp' Dockerfile.bench

fail_found "bench CLI special evidra mode flag registrations" \
  rg_repo -n -e '(StringVar|BoolVar|StringSliceVar|StringP|BoolP)[^(]*\([^)]*"(evidra-bin|evidra-evidence-dir|proxy-mode|smart-prescribe|trace|evidra)"' \
    --glob '!**/*_test.go' cmd pkg internal

fail_found "active code still exposes Evidra-named bench API contract" \
  rg_repo -n -e '(EVIDRA_[A-Z0-9_]+|VITE_EVIDRA_API|--evidra-url|--evidra-api-key|EvidraURL|EvidraAPIKey|X-Evidra-Tenant|evidra_url|evidra_api_key)' \
    --glob '!**/*_test.go' \
    --glob '!**/*.test.mts' \
    --glob '!**/*.test.ts' \
    --glob '!**/*.test.tsx' \
    cmd pkg internal profiles scripts ui/src ui/Dockerfile .github Dockerfile.bench

fail_found "active Evidra protocol verifier surface" \
  rg_repo -n -e '(EvidraExpectations|BuildEvidraCheckers|EvidraCheckConfig|evidra-protocol|evidra_enabled)' \
    --glob '!**/*_test.go' \
    --glob '!**/*.test.mts' \
    --glob '!**/*.test.ts' \
    --glob '!**/*.test.tsx' \
    cmd pkg internal ui/src scenarios

fail_found "scenario-level evidra expectations" \
  rg_repo -n -e '^evidra:' scenarios

fail_found "bench-owned evidra-mcp special mode" \
  rg_repo -n -e 'evidra-mcp|Evidra MCP|evidra mcp' \
    --glob '!docs/archive/**' \
    --glob '!docs/backlog/**' \
    --glob '!docs/ideas/**' \
    --glob '!docs/plans/**' \
    --glob '!**/*_test.go' \
    --glob '!**/*.test.mts' \
    --glob '!**/*.test.ts' \
    --glob '!**/*.test.tsx' \
    cmd pkg internal scripts ui/src docs README.md
