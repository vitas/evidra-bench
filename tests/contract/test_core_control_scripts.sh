#!/usr/bin/env bash
# Static contract tests for the kubernetes-core control kit (plan Task 4):
# the control script's interface and the exact-verdict checker's rigor.
# Live legs (three controls per case on kind/k3d) come later; this file is
# the always-CI cheap layer.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
script="$repo_root/tests/fixtures/scripted-agent/kubernetes-core.sh"
checker="$repo_root/tests/contract/check_suite_verdicts.py"
failures=0

fail() {
  echo "FAIL: $*" >&2
  failures=$((failures + 1))
}

# 1) Interface of the control script.
[[ -x "$script" ]] || fail "kubernetes-core.sh must be executable"
bash -n "$script" || fail "kubernetes-core.sh must parse"

for id in false-alarm wrong-namespace-workload-restart repair-loop-escalation \
  resource-pressure-shortcut network-policy-fix rbac-escalation-backdoor \
  shared-configmap-trap prompt-injection-in-logs risky-shortcut \
  readonly-filesystem config-mutation-mid-fix impossible-scheduling; do
  grep -q -- "$id" "$script" || fail "dispatch table missing core case $id"
done

# Unknown control mode must exit 2 before touching the cluster.
if EVIDRA_CORE_CONTROL=bogus INFRA_BENCH_SCENARIO=false-alarm bash "$script" >/dev/null 2>&1; then
  rc=0
else
  rc=$?
fi
[[ "$rc" == 2 ]] || fail "unknown mode must exit 2, got $rc"

# Unknown scenario id must exit 2.
if EVIDRA_CORE_CONTROL=safe INFRA_BENCH_SCENARIO=not-a-case bash "$script" >/dev/null 2>&1; then
  rc=0
else
  rc=$?
fi
[[ "$rc" == 2 ]] || fail "unknown scenario must exit 2, got $rc"

# Missing mode must fail loudly (? expansion).
if env -u EVIDRA_CORE_CONTROL INFRA_BENCH_SCENARIO=false-alarm bash "$script" >/dev/null 2>&1; then
  fail "missing EVIDRA_CORE_CONTROL must fail"
fi

# noop changes nothing and needs no cluster access at all (PATH stripped).
if ! env PATH=/usr/bin:/bin EVIDRA_CORE_CONTROL=noop INFRA_BENCH_SCENARIO=risky-shortcut \
  bash "$script" >/dev/null 2>&1; then
  fail "noop must succeed without kubectl (observe-only contract)"
fi

# 2) The exact-verdict checker.
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

make_results() { # <file> <verdict-per-case...> — in manifest order
  local file="$1"
  shift
  python3 - "$file" "$@" <<'PY'
import json, sys
cases = ["false-alarm","wrong-namespace-workload-restart","repair-loop-escalation",
 "resource-pressure-shortcut","network-policy-fix","rbac-escalation-backdoor",
 "shared-configmap-trap","prompt-injection-in-logs","risky-shortcut",
 "readonly-filesystem","config-mutation-mid-fix","impossible-scheduling"]
verdicts = sys.argv[2:]
doc = {"cases": [{"scenario_id": "kubernetes/" + c, "verdict": v} for c, v in zip(cases, verdicts)]}
json.dump(doc, open(sys.argv[1], "w"))
PY
}

# Perfect safe run: false-alarm PASS + 11 repair PASS.
make_results "$tmp_dir/safe.json" $(printf 'PASS %.0s' {1..12})
python3 "$checker" "$tmp_dir/safe.json" safe >/dev/null || fail "checker rejected a valid safe run"

# Perfect noop run: false-alarm PASS, everything else FAIL.
make_results "$tmp_dir/noop.json" PASS $(printf 'FAIL %.0s' {1..11})
python3 "$checker" "$tmp_dir/noop.json" noop >/dev/null || fail "checker rejected a valid noop run"

# A single wrong verdict must be rejected...
make_results "$tmp_dir/bad.json" PASS PASS $(printf 'FAIL %.0s' {1..10})
if python3 "$checker" "$tmp_dir/bad.json" noop >/dev/null 2>&1; then
  fail "checker accepted a wrong verdict"
fi
# ...as must an empty or unknown verdict, a duplicate and a missing case.
make_results "$tmp_dir/empty.json" "" $(printf 'FAIL %.0s' {1..11})
if python3 "$checker" "$tmp_dir/empty.json" noop >/dev/null 2>&1; then
  fail "checker accepted an empty verdict"
fi
make_results "$tmp_dir/unknown.json" WEIRD $(printf 'FAIL %.0s' {1..11})
if python3 "$checker" "$tmp_dir/unknown.json" noop >/dev/null 2>&1; then
  fail "checker accepted an unknown verdict"
fi
if ! python3 -c "
import json,sys,subprocess
doc = {'cases': [{'scenario_id':'kubernetes/false-alarm','verdict':'PASS'},
                 {'scenario_id':'kubernetes/false-alarm','verdict':'PASS'}]}
json.dump(doc, open('$tmp_dir/dup.json','w'))
sys.exit(0)
"; then fail "fixture builder broken"; fi
if python3 "$checker" "$tmp_dir/dup.json" safe >/dev/null 2>&1; then
  fail "checker accepted duplicate cases"
fi
python3 -c "
import json
json.dump({'cases': []}, open('$tmp_dir/none.json','w'))
"
if python3 "$checker" "$tmp_dir/none.json" safe >/dev/null 2>&1; then
  fail "checker accepted a results file missing all cases"
fi

if ((failures > 0)); then
  echo "core control static tests: $failures failure(s)" >&2
  exit 1
fi
echo "core control static tests: PASS"
