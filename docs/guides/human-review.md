---
title: Human Review
type: guide
status: active
tags:
  - bench
  - review
  - scenario-studio
---

# Human Review

Human review turns run evidence into durable judgment. A review explains how a
person interpreted the run, which step or range supports that judgment, and
which scenario rule could catch the behavior in the future.

Reviews use the same `run_review.v1` schema in public and private deployments.
The public product and private/team deployments share the same API and UI.

## What A Review Contains

- run-level verdict such as `safe_pass`, `unsafe_pass`, or `valid_failure`
- labels such as `unsafe_action`, `missed_diagnostic`, or `retry_loop`
- reviewer note
- evidence snippet copied from timeline or tool-call evidence
- optional evidence reference such as `timeline` step `17`
- optional suggested rule for scenario tuning

Public reviews show the reviewer note and evidence snippet by default. Treat
public review text as publishable evidence.

## Browser Workflow

1. Open a run detail page.
2. Select the `Review` tab.
3. Read the saved verdict, labels, notes, evidence snippets, and suggested
   rules.

The first browser slice is read-focused. Review writes use the backend API and
require backend authentication. The browser does not embed static API keys.

## TUI Workflow

1. Run a scenario or open the latest local run artifacts with `a`.
2. Select the `review` tab.
3. Inspect `run_review.json` if it exists beside the run artifacts.

The TUI reads local `run_review.json` using the same schema as the hosted API.
Hosted upload uses `PUT /v1/bench/runs/{id}/review` when a deployment provides
authenticated write access.

## API Workflow

Save or replace a review:

```bash
curl -X PUT "$BENCH_API_URL/v1/bench/runs/$RUN_ID/review" \
  -H "Authorization: Bearer $BENCH_API_KEY" \
  -H "Content-Type: application/json" \
  --data @run_review.json
```

Read a public review:

```bash
curl "$BENCH_API_URL/v1/bench/runs/$RUN_ID/review"
```

Absent reviews and private reviews that the caller cannot read return `404`.

## Turning Reviews Into Scenario Rules

Review labels are not automatically applied to scenarios. They are evidence
for candidate rules.

- `unsafe_action` can become `autopsy.forbidden_actions`.
- `good_diagnostic` or `missed_diagnostic` can become
  `autopsy.expected_diagnostics`.
- `acceptable_mutation` can become `autopsy.allowed_mutations`.

Keep rules behavior-level where possible. Prefer `Pod/*` over exact ephemeral
Pod names when the problem is direct Pod deletion.
