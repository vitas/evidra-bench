# Exporting Bench Runs as Evidra Evidence Bundles

Every `evidra test` run already exports its bundles automatically: signed
evidence for each run lands in `<output>/bundles/<run-id>` (by default
`./evidra-results/bundles/`). The manual command below remains for exporting
older archived runs.

`bench-cli export-bundle` converts one benchmark run's artifacts into an
**Evidra external evidence bundle** (`evidra-external-bundle/v1`): the same
append-only, signature-chained store format that the Evidra flight recorder
writes for production agents. A bundle can be opened by anyone with the
Evidra CLI — no server, no account:

```bash
# export a finished run
bin/bench-cli export-bundle --run 01SMOKE... --out ./my-run-bundle
# or point at the artifact directory directly
bin/bench-cli export-bundle --run-dir ./runs/artifacts/<run-id> --out ./my-run-bundle

# consume with the Evidra flight recorder
evidra validate  --evidence-dir ./my-run-bundle   # chain + signatures
evidra scorecard --evidence-dir ./my-run-bundle   # signals + score
```

## What lands in the bundle

A v1 export maps one run to five chained entries:

| # | Entry | Content |
|---|---|---|
| 1 | `session_start` | run/scenario/adapter/model labels, duration, metadata |
| 2 | `prescribe` | declared intent of the task (adapter, scenario, prompt digest inputs) |
| 3 | `report` | exit code + verdict (`VerdictFromExitCode`), links to the prescription |
| 4 | `annotation` | `evidra-bench/run_summary`: tool-call count, verifier check tally, chaos flag |
| 5 | `session_end` | completion status |

Entry timestamps reflect **export time** (that is when the evidence was
authored); the original run window lives in the `session_start` labels.
Per-tool-call `prescribe`/`report` pairs are planned for v2 — they need
per-call exit codes recorded by adapters first. Until then the annotation
carries the coarse tally.

## Cohorts

`bundle.json` records the `semantics_version` of the run it came from.
Bundles exported before ADR 0001 report no version and decode as legacy
`preview-v1`: always individually verifiable. The tag is provenance — it
tells the evidence eras apart — while the certification-era rule that
REFUSED mixed-cohort joins was removed with that layer; treat a preview-era
verdict and an evidence-era verdict as different contracts when you
compare them. Verify any bundle with `evidra validate --evidence-dir
<dir>`; see [ADR 0001](adr/0001-process-safety-matching.md).

## Integrity and trust

- The bundle is signed with an **ephemeral Ed25519 key** generated per export;
  the public half is embedded in `bundle.json`, the private half is discarded.
  `evidra validate` verifies every entry hash, the chain links, and every
  signature against it automatically.
- Trust level is `ephemeral`: this proves nobody edited the bundle after the
  export and that all entries were produced atomically by the same process. It
  does **not** attest who ran the export — that is inherent to local tools and
  the reason hosted/verified signing (a long-lived producer key) is a separate
  future step.

## Protocol location

The wire format lives in Evidra core
([docs/external-evidence-bundle-v1.md](https://github.com/vitas/evidra/blob/main/docs/external-evidence-bundle-v1.md)).
This repo carries a pinned, byte-checked copy of the minimal producer surface
in `pkg/evidrawire/` (chosen over a module dependency because the
`samebits.com/evidra` vanity import is not yet resolvable by `go get`). Drift
is caught by `TestCoreBundleFixtureReproducesHashes`, which re-hashes the
upstream conformance fixture committed under `pkg/evidrawire/testdata/`.
When the core module becomes importable, replace `pkg/evidrawire` with the
real dependency; the fixture test is the migration safety net.
