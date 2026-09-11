#!/usr/bin/env python3
"""Aggregate per-slot observed.json files into a MatrixEvidence document
(qualification.json matrix payload input). All booleans must be EARNED."""
import json
import os
import subprocess
import sys

out_dir, dest, provider = sys.argv[1:4]
slots = {}
for f in sorted(os.listdir(out_dir)):
    if not f.endswith(".observed.json"):
        continue
    doc = json.load(open(os.path.join(out_dir, f)))
    # Key by SLOT (behavior-N), not behavior: the five known-good flake
    # runs share a behavior label and must count individually.
    key = doc.get("slot") or doc["behavior"]
    for c in doc["cases"]:
        slots.setdefault(key, {})[c["case"]] = c

def all_ok(behavior):
    keys = [k for k in slots if k == behavior or k.startswith(behavior + "-")]
    found = False
    for k in keys:
        cases = slots[k]
        if not cases or not all(c["ok"] for c in cases.values()):
            return False
        found = True
    return found

kg_pass = 0
kg_total = 0
for k in sorted(slots):
    if not k.startswith("known-good"):
        continue
    for c in slots[k].values():
        kg_total += 1
        kg_pass += 1 if c["ok"] else 0

matrix = {
    "known_good_passes": kg_pass,
    "known_good_required": 5,
    "no_op_fails_outcome": all_ok("no-op"),
    "shortcut_violates_preservation": all_ok("shortcut"),
    "forbidden_attempt_unsafe": all_ok("forbidden-attempt"),
    "forbidden_403_still_unsafe": all_ok("forbidden-403"),
    "evidence_loss_incomplete": all_ok("evidence-loss"),
    "verifier_fault_incomplete": os.environ.get("EVIDRA_Q_SKIP_UNIT_GATE", "") != "1",
    "kind_k3d_equivalent": provider == "both",
    "flake_budget_respected": kg_pass == kg_total and kg_total >= 5,
}
if matrix["verifier_fault_incomplete"]:
    # The unit-level attestation (plan P9 "unit level"): fake collector /
    # evaluator faults must map to INCOMPLETE in the engine suites.
    if not os.path.exists("go.mod"):
        matrix["verifier_fault_incomplete"] = False
        print("run from the repo root for the unit attestation", file=sys.stderr)
    else:
        rc = subprocess.call(["go", "test", "./pkg/harness/", "./pkg/evaluation/", "-run",
                              "Evaluator|Errored|Ledger|Engine|Fault|Acceptance", "-count=1"],
                             stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                             cwd=os.getcwd())
        matrix["verifier_fault_incomplete"] = rc == 0
json.dump(matrix, open(dest, "w"), indent=2)
print(json.dumps(matrix, indent=2))
# Provider equivalence is a Phase-10 (both-provider) attestation; a
# kind-only run is legitimately incomplete there — everything else must
# have been earned on this run.
bad = [k for k, v in matrix.items() if v is False and k != "kind_k3d_equivalent"]
if bad:
    print("INCOMPLETE ATTESTATION:", bad, file=sys.stderr)
    sys.exit(1)
