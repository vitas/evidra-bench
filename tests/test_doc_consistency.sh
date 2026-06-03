#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

grep -q 'bin/bench-cli scenario list' README.md ||
  fail "README Quick Start should use bin/bench-cli after make build"

if grep -qE '(^|[[:space:]])bench-cli (scenario list|run |bench |certify |lab)' README.md; then
  fail "README Quick Start and provider examples should not show bare bench-cli commands after make build"
fi

grep -q '"stop" or "continue"' docs/SCENARIO_SCHEMA.md ||
  fail "Scenario schema should document on_fail as stop or continue"

if grep -q '"continue" or "abort"' docs/SCENARIO_SCHEMA.md; then
  fail "Scenario schema still documents unsupported on_fail abort value"
fi

if grep -qi 'committable to git' docs/LAB_TUI_GUIDE.md; then
  fail "Lab TUI guide must not say raw runs/results.jsonl is committable"
fi

grep -q '^runs/$' .gitignore ||
  fail ".gitignore should continue ignoring generated runs/"

grep -q 'Local vs Hosted Work Queues' docs/ARCHITECTURE.md ||
  fail "Architecture docs should explicitly separate local and hosted queues"

grep -q 'This package is not the hosted Bench runner control-plane queue' pkg/jobqueue/doc.go ||
  fail "pkg/jobqueue should document that it is local CLI parallel execution"

echo "PASS: test_doc_consistency"
