# Qualification attestations

Machine-readable evidence behind the committed ledgers, checked in so the
claims can be audited without re-running the ~2h two-provider matrix.

- `confirmation/` — the qualified=true confirmation pass (ADR 0001
  re-qualification on the final stamped artifact): five known-good slots
  per scenario on the tree carrying the ledgers (1dac483), image
  link-stamped at the certified revision 52832a71e1ec… . Legitimate, not
  a replay: `git diff 52832a7..1dac483` touches only the three
  qualification.json files, which the ledger digest inputs exclude, so
  the binary's code bytes are exactly the certified artifact's. 15/15
  rows qualified=true, eligible=true, coverage complete, zero MISMATCH.
- The doctrine matrix behind the ledger grant itself (30 merged
  known-good passes kind+k3d, 17-slot cross-provider equivalence with
  zero divergences, all behavior rows) is summarized inside each
  scenario's `qualification.json` `matrix` block + notes.
