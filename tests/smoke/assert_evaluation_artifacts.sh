#!/usr/bin/env bash
# Shared assertions for the canonical artifacts of one evaluation run:
# result.json, report.html, and signed evidence bundles. Sourced by the
# Docker smokes; must not encode provider-specific expectations.
#
# Usage: assert_evaluation_artifacts <result_dir>

assert_evaluation_artifacts() {
  local result_dir="$1"
  test -s "$result_dir/result.json" || { echo "missing $result_dir/result.json" >&2; return 1; }
  test -s "$result_dir/report.html" || { echo "missing $result_dir/report.html" >&2; return 1; }
  grep -Eq '"passed": 3' "$result_dir/result.json" ||
    { echo "expected 3 passed cases in $result_dir/result.json" >&2; return 1; }
  grep -Eq '"failed": 0' "$result_dir/result.json" ||
    { echo "expected 0 failed cases" >&2; return 1; }
  grep -Eq '"unsafe": 0' "$result_dir/result.json" ||
    { echo "expected 0 unsafe cases" >&2; return 1; }
  grep -Eq '"incomplete": 0' "$result_dir/result.json" ||
    { echo "expected 0 incomplete cases" >&2; return 1; }
  local bundle_count
  bundle_count="$(find "$result_dir/bundles" -name bundle.json 2>/dev/null | wc -l | tr -d ' ')"
  if [[ "$bundle_count" -eq 0 ]]; then
    echo "expected signed evidence bundles under $result_dir/bundles" >&2
    return 1
  fi
}
