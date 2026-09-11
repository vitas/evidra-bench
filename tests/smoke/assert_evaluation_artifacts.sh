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

  # ADR 0001 Phase 5+: the api_audit source must report COMPLETE coverage
  # with both window markers observed.
  grep -Eq '"coverage": *"complete"' "$result_dir/result.json" ||
    { echo "expected api_audit coverage complete in result.json" >&2; return 1; }
  grep -Eq '"name": *"api_audit"' "$result_dir/result.json" ||
    { echo "expected api_audit source entry in result.json" >&2; return 1; }
  # ADR 0001 Phase 10+: these smokes execute the agent UNCONFINED (scripted
  # --agent), so qualified=false is a structural truth, not a preview
  # disclaimer: confinement is a hard prerequisite of qualification. Assert
  # the labeling itself, so an accidental flip without a sandbox would fail
  # the smoke loudly.
  grep -Eq '"qualified": *false' "$result_dir/result.json" ||
    { echo "unconstrained runs must never report qualified=true" >&2; return 1; }
  grep -Eq '"unconfined": *true' "$result_dir/result.json" ||
    { echo "expected unconfined runtime labeling in result.json" >&2; return 1; }
  grep -Eq 'agent_unconfined' "$result_dir/result.json" ||
    { echo "expected agent_unconfined gap for --agent runs" >&2; return 1; }
  # ADR 0001 Phase 11: cohort stamp — every current-binary run belongs to
  # safety-evidence.v1; nothing in this repo may emit unstamped results.
  grep -Eq '"semantics_version": *"safety-evidence.v1"' "$result_dir/result.json" ||
    { echo "expected safety-evidence.v1 semantics stamp" >&2; return 1; }
  grep -Eq '"engine"' "$result_dir/result.json" ||
    { echo "expected authoritative engine block under safety in result.json" >&2; return 1; }
  local audit_files
  audit_files="$(find "$result_dir/runs" -name audit.jsonl 2>/dev/null | wc -l | tr -d ' ')"
  if [[ "$audit_files" -lt "$bundle_count" ]]; then
    echo "expected audit.jsonl in every run dir (bundles=$bundle_count audit=$audit_files)" >&2
    return 1
  fi
  # Window markers must be inside the persisted evidence.
  grep -rq "evidra-marker-" "$result_dir/runs" || {
    echo "no window marker events found in audit evidence" >&2
    return 1
  }

  # ADR 0001 Phase 7: the state_snapshot source (evidence-reader checkpoints
  # + preservation diff) must also report complete coverage on every case,
  # with the normalized (redacted) checkpoints persisted per run.
  grep -Eq '"name": *"state_snapshot"' "$result_dir/result.json" ||
    { echo "expected state_snapshot source entry in result.json" >&2; return 1; }
  local complete_sources
  complete_sources="$(grep -c '"coverage": *"complete"' "$result_dir/result.json")"
  if [[ "$complete_sources" -lt $((bundle_count * 2)) ]]; then
    echo "expected 2 complete sources per run (audit + snapshot), got $complete_sources for $bundle_count runs" >&2
    return 1
  fi
  local snap_files
  snap_files="$(find "$result_dir/runs" -name 'snapshot-*.json' 2>/dev/null | wc -l | tr -d ' ')"
  if [[ "$snap_files" -lt $((bundle_count * 3)) ]]; then
    echo "expected 3 snapshot checkpoints per run dir, got $snap_files for $bundle_count runs" >&2
    return 1
  fi
  # Secret canary: no plaintext may survive in persisted evidence artifacts.
  if grep -rqs "ZXZpZHJhLWNhbmFyeQ" "$result_dir/runs" 2>/dev/null; then
    echo "redaction canary leaked into run evidence" >&2
    return 1
  fi
}
