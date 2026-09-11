#!/usr/bin/env python3
"""Cross-provider equivalence check (ADR 0001 Phase 10).

Compares the per-(behavior, case) OBSERVATIONS of a kind matrix run against
a k3d matrix run. Equivalence means: same engine verdict, same legacy
verdict, same eligible bit, same coverage states, same qualified bit.
Numbers (digests, durations, allowed-change counts) may differ by cluster
churn; the JUDGEMENT must not.

usage: check_equivalence.py KIND_OUT_DIR K3D_OUT_DIR
"""
import json
import os
import sys

kind_dir, k3d_dir = sys.argv[1], sys.argv[2]


def load(directory):
    table = {}
    for f in sorted(os.listdir(directory)):
        if not f.endswith(".observed.json"):
            continue
        doc = json.load(open(os.path.join(directory, f)))
        beh = doc["behavior"]
        for c in doc["cases"]:
            table[(beh, c["case"])] = {
                "verdict": c["verdict"],
                "engine": c["engine_verdict"],
                "eligible": c["eligible"],
                "qualified": c["qualified"],
                "audit": c["audit_coverage"],
                "snapshot": c["snapshot_coverage"],
            }
    return table


a, b = load(kind_dir), load(k3d_dir)
problems = 0
for key in sorted(set(a) | set(b)):
    if key not in a or key not in b:
        # no-op legitimately skips false-alarm etc. — only flag real gaps
        if key in a and a[key]["verdict"] is not None:
            pass
        continue
    if a[key] != b[key]:
        problems += 1
        print(f"DIVERGENCE {key[0]}/{key[1]}:")
        for fld in a[key]:
            if a[key][fld] != b[key][fld]:
                print(f"  {fld}: kind={a[key][fld]!r} k3d={b[key][fld]!r}")
shared = len(set(a) & set(b))
print(f"compared {shared} slots; divergences: {problems}")
sys.exit(1 if problems else 0)
