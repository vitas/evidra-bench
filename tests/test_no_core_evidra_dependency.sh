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

fail_found "go.mod require/replace samebits.com/evidra" \
  grep -nE 'samebits\.com/evidra([[:space:]]|$)' go.mod

fail_found "Go imports from core evidra" \
  grep -R -nF --include='*.go' --exclude-dir='.git' '"samebits.com/evidra/' .

fail_found "CI/release checkout of core evidra" \
  grep -R -nE 'Checkout evidra|repository: .*/evidra$' .github/workflows

fail_found "Dockerfile.bench cloning/building core evidra" \
  grep -nE 'EVIDRA_REPO|EVIDRA_REF|/build/parent|cmd/evidra-mcp' Dockerfile.bench

fail_found "bench CLI special evidra mode flag registrations" \
  grep -R -nE --exclude='*_test.go' '(StringVar|BoolVar|StringSliceVar|StringP|BoolP)[^(]*\([^)]*"(evidra-bin|evidra-evidence-dir|proxy-mode|smart-prescribe|trace|evidra)"' \
    cmd pkg internal

fail_found "active code still exposes Evidra-named bench API contract" \
  grep -R -nE --exclude='*_test.go' --exclude='*.test.mts' --exclude='*.test.ts' --exclude='*.test.tsx' \
    '(EVIDRA_[A-Z0-9_]+|VITE_EVIDRA_API|--evidra-url|--evidra-api-key|EvidraURL|EvidraAPIKey|X-Evidra-Tenant|evidra_url|evidra_api_key)' \
    cmd pkg internal profiles scripts ui/src ui/Dockerfile .github Dockerfile.bench

fail_found "active Evidra protocol verifier surface" \
  grep -R -nE --exclude='*_test.go' --exclude='*.test.mts' --exclude='*.test.ts' --exclude='*.test.tsx' \
    '(EvidraExpectations|BuildEvidraCheckers|EvidraCheckConfig|evidra-protocol|evidra_enabled)' \
    cmd pkg internal ui/src scenarios

fail_found "scenario-level evidra expectations" \
  grep -R -nE '^evidra:' scenarios

fail_found "bench-owned evidra-mcp special mode" \
  grep -R -nE --exclude-dir='archive' --exclude-dir='backlog' --exclude-dir='ideas' --exclude-dir='plans' \
    --exclude='*_test.go' --exclude='*.test.mts' --exclude='*.test.ts' --exclude='*.test.tsx' \
    'evidra-mcp|Evidra MCP|evidra mcp' \
    cmd pkg internal scripts ui/src docs README.md
