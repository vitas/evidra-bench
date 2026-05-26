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

1. Open `Reviews` from the bench navigation.
2. Triage the `Needs Review`, `Unsafe Passes`, and `Reviewed Failures` queues.
3. Open a run from the queue or from the runs table.
4. Select the `Review` tab.
5. Read the saved verdict, labels, notes, evidence snippets, and suggested
   rules.
6. Use the review editor to set the verdict, visibility, primary label,
   severity, reviewer note, evidence snippet, and optional suggested rule.
7. Save the review to replace the current `run_review.v1` artifact.

Browser review writes use the existing `PUT /v1/bench/runs/{id}/review`
backend API and require backend authentication. The browser does not embed
static API keys. In private deployments, open `Session`, sign in with the
deployment API key once, and the backend sets an HttpOnly session cookie.
Serve the UI and API from the same site or behind the same reverse proxy for
browser session writes.
Run list and run detail responses include a compact `review_summary` when a
saved review is readable. Anonymous public reads only show summaries for public
reviews; authenticated deployments can show private review summaries for the
current tenant. The browser review queues use backend run filters rather than
client-side filtering over a fixed recent-run sample.

Useful queue filters:

- `GET /v1/bench/runs?review=unreviewed`
- `GET /v1/bench/runs?passed=true&review_verdict=unsafe_pass`
- `GET /v1/bench/runs?passed=false&review=reviewed`

## TUI Workflow

1. Run a scenario or open the latest local run artifacts with `a`.
2. Press `r` from the artifact view to open the review editor.
3. Choose the evidence step, verdict, label kind, severity, visibility, and
   reviewer note.
4. Save local `run_review.json` with `w`.
5. Use `u` to save and upload when the deployment provides authenticated write
   access.

The editor uses the same `run_review.v1` schema as the hosted API. It selects
the first mutation step by default, fills the evidence snippet from timeline
evidence, and creates a reviewer note by default. Hosted upload uses
`PUT /v1/bench/runs/{id}/review` with `BENCH_API_URL`/`BENCH_API_KEY` or the
TUI lab config `bench_url`/`bench_api_key`.

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
