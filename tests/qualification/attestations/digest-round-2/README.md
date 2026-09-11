# Round-2 re-qualification attestation (content-digest era)

Certified artifact: `code-02fb086cc35a821cb4a718e63a35dc7a25279d6d`
(tools/code-revision.sh over *.go, go.mod, go.sum — the ledger identity
is now the EXECUTABLE content, so the commit carrying these ledgers
satisfies them; see blocker #2 of the owner review round 2).

Both provider legs ran on a clean tree at 8e49d09 (image built from that
tree, stamped with the digest above):

- kind leg: 15 known-good slots (5 flake × 3 scenarios), no-op, shortcut,
  forbidden-attempt, forbidden-403, evidence-loss — all rows green,
  zero MISMATCH lines (observed JSON stdout pure);
- k3d leg: same roster, same result;
- cross-provider equivalence: 17 slots compared, 0 divergences;
- merged attestation (what the ledgers embed): merged-matrix.json
  (known_good_passes 30 / required 5, all doctrine flags true).

The legs' commit attestations are kept verbatim; bundles for full tree
forensics live in the runner work dirs (/tmp/qGkind, /tmp/qGk3d).

## Confirmation on the ledger-carrying tree (no PRE_RECORD)

Run leg: commit `a2c5714255f9` (this tree — it already carries the three
ledgers), image stamped `code-02fb086cc35a` — and `git diff a2c5714
8e49d09` over `*.go,go.mod,go.sum` is empty: the binary's code is
byte-identical to what both matrices certified. This is the property the
digest identity buys: **the artifact satisfies its own ledger on the
committed tree**, without softening anything (EVIDRA_Q_PRE_RECORD unset).

`observed/` holds all five known-good slots: 15/15 rows
`qualified=true` with `qualified_pending_record` absent, coverage
complete, zero MISMATCH. The same assertion now runs in CI on every push
(qualification smoke, both providers) — the gate-opening is no longer a
local claim, it is a release-blocking check.
