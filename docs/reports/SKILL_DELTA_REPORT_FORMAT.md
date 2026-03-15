# Skill Delta Report Format

## Files

`infra-bench skill-delta aggregate` and `infra-bench skill-delta report` write
three top-level artifacts under one benchmark directory:

- `benchmark.json`
- `benchmark.md`
- `benchmark.html`

Per-case normalized rows live at:

- `cases/<scenario>/<model>/repeat-N/pair.json`

## `pair.json`

One paired comparison row for a single scenario, model, and repeat. It includes:

- `without_skill` and `with_skill` run snapshots
- pass/fail outcome and `verdict_delta`
- duration, token, cost, compliance, and score deltas
- protocol counters such as prescribe/report counts and declined verdicts
- optional scorecard metrics and signal counts
- local artifact paths back to the original run outputs

## `benchmark.json`

Machine-readable aggregate output for tooling and future API integration.

Top-level sections:

- `metadata`
- `pairs`
- `summary`

`summary` stores configuration-level means, sample standard deviations, and
`with_skill - without_skill` deltas for:

- pass rate
- compliance rate
- duration
- total tokens
- estimated cost
- Evidra score

## `benchmark.md`

Concise review artifact for pull requests, notes, and terminal-friendly review.

It contains:

- one summary table
- one per-pair comparison table

The Markdown output is deterministic so it can be diffed cleanly across runs.

## `benchmark.html`

Standalone static report generated locally by Go.

Sections:

- summary cards
- aggregate table
- filterable per-pair table
- direct links back to `without_skill` and `with_skill` artifact directories

The report has no server dependency and ships with embedded CSS and JavaScript.
