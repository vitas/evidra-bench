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

fail_found "active code still exposes Evidra-named bench API contract" \
  sh -c "git grep -nE '(EVIDRA_[A-Z0-9_]+|VITE_EVIDRA_API|--evidra-url|--evidra-api-key|EvidraURL|EvidraAPIKey|X-Evidra-Tenant|evidra_url|evidra_api_key)' -- cmd pkg internal profiles scripts ui/src ui/Dockerfile .github Dockerfile.bench ':(exclude)*_test.go' ':(exclude)*.test.mts' ':(exclude)*.test.ts' ':(exclude)*.test.tsx'"

fail_found "active Evidra protocol verifier surface" \
  sh -c "git grep -nE '(EvidraExpectations|BuildEvidraCheckers|EvidraCheckConfig|evidra-protocol|evidra_enabled)' -- cmd pkg internal ui/src scenarios ':(exclude)*_test.go' ':(exclude)*.test.mts' ':(exclude)*.test.ts' ':(exclude)*.test.tsx'"

fail_found "scenario-level evidra expectations" \
  sh -c "git grep -nE '^evidra:' -- scenarios"

fail_found "bench-owned evidra-mcp special mode" \
  sh -c "git grep -nE 'evidra-mcp' -- cmd pkg internal scripts ui/src docs README.md CLAUDE.md ':(exclude)docs/archive/**' ':(exclude)docs/backlog/**' ':(exclude)docs/ideas/**' ':(exclude)docs/plans/**' ':(exclude)*_test.go' ':(exclude)*.test.mts' ':(exclude)*.test.ts' ':(exclude)*.test.tsx'"
