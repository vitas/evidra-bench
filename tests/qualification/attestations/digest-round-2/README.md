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
