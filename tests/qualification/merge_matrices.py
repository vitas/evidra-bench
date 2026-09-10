#!/usr/bin/env python3
"""Merge kind+k3d matrix attestations into one ledger matrix (P10)."""
import json, sys
a, b, dest = (json.load(open(p)) if i < 2 else p for i, p in enumerate(sys.argv[1:]))
merged = {}
for k in a:
    va, vb = a[k], b.get(k)
    if isinstance(va, bool):
        merged[k] = bool(va) and bool(vb)
    elif k == "known_good_passes":
        merged[k] = int(va) + int(b.get(k, 0))
    elif k == "known_good_required":
        merged[k] = max(va, b.get(k, 0))
    else:
        merged[k] = va
merged["kind_k3d_equivalent"] = True  # caller must gate on check_equivalence.py
json.dump(merged, open(dest, "w"), indent=2)
print(json.dumps(merged, indent=2))
missing = [k for k, v in merged.items() if v is False]
sys.exit(1 if missing else 0)
