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
2. Choose a run from `Needs Review`, `Unsafe Passes`, `Reviewed Failures`, or
   the runs table.
3. Open the run.
4. Select the `Review` tab.
5. Read the saved verdict, labels, notes, evidence snippets, and suggested
   rules.
6. Optionally select `Draft with AI` to fill the editor from stored autopsy,
   timeline, and tool-call artifacts.
7. Use the review editor to set the verdict, visibility, primary label,
   severity, reviewer note, evidence snippet, and optional suggested rule.
8. Save the review to replace the current `run_review.v1` artifact.
9. If the saved review contains suggested rules, select `Preview scenario
   patch` to inspect the exact scenario YAML diff before changing the scenario.
10. When the preview produces a diff, select `Download diff` to save a clean
    unified diff for manual review or application outside the browser.

Browser review writes use the existing `PUT /v1/bench/runs/{id}/review`
backend API and require backend authentication. The browser does not embed
static API keys. In private deployments, open `Session`, sign in with the
deployment API key once, and the backend sets an HttpOnly session cookie.
Serve the UI and API from the same site or behind the same reverse proxy for
browser session writes.
After deployment, validate the path with `make private-review-smoke` against a
dedicated smoke run.
Run list and run detail responses include a compact `review_summary` when a
saved review is readable. Anonymous public reads only show summaries for public
reviews; authenticated deployments can show private review summaries for the
current tenant. The browser review queues use backend run filters rather than
client-side filtering over a fixed recent-run sample.

Useful queue filters:

- `GET /v1/bench/review-candidates`
- `GET /v1/bench/runs?passed=true&review_verdict=unsafe_pass`
- `GET /v1/bench/runs?passed=false&review=reviewed`
- `GET /v1/bench/scenario-improvements`

The browser `Needs Review` queue uses `GET /v1/bench/review-candidates`.
That endpoint returns unreviewed runs with artifact coverage, autopsy-derived
signals, priority, a human-readable reason, and a draft action URL. Opening a
candidate preloads an unsaved artifact-derived draft in the review editor.

The browser `Scenario Improvements` queue uses `GET
/v1/bench/scenario-improvements`. That endpoint returns reviewed runs with
suggested rules as product-level candidates, including reviewer note, evidence
snippet, suggested rule count, and action URLs for reading the review or
building, reading, and downloading a scenario patch preview.

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

Build an unsaved artifact-derived draft:

```bash
curl -X POST "$BENCH_API_URL/v1/bench/runs/$RUN_ID/review-draft" \
  -H "Authorization: Bearer $BENCH_API_KEY"
```

Build the same draft from the review-candidates namespace:

```bash
curl -X POST "$BENCH_API_URL/v1/bench/review-candidates/$RUN_ID/draft" \
  -H "Authorization: Bearer $BENCH_API_KEY"
```

Preview a scenario patch from the saved review:

```bash
curl -X POST "$BENCH_API_URL/v1/bench/runs/$RUN_ID/scenario-patch-preview" \
  -H "Authorization: Bearer $BENCH_API_KEY"
```

Read the stored preview artifact or download the raw diff later:

```bash
curl "$BENCH_API_URL/v1/bench/runs/$RUN_ID/scenario-patch-preview" \
  -H "Authorization: Bearer $BENCH_API_KEY"

curl "$BENCH_API_URL/v1/bench/runs/$RUN_ID/scenario-patch.diff" \
  -H "Authorization: Bearer $BENCH_API_KEY" \
  -o scenario.patch.diff
```

Queue a validation rerun after applying the diff:

```bash
curl -X POST "$BENCH_API_URL/v1/bench/runs/$RUN_ID/scenario-patch-validation" \
  -H "Authorization: Bearer $BENCH_API_KEY"
```

Read the durable validation record, including trigger status and validation
run IDs when available:

```bash
curl "$BENCH_API_URL/v1/bench/runs/$RUN_ID/scenario-patch-validation" \
  -H "Authorization: Bearer $BENCH_API_KEY"
```

Read a public review:

```bash
curl "$BENCH_API_URL/v1/bench/runs/$RUN_ID/review"
```

Absent reviews and private reviews that the caller cannot read return `404`.

## Turning Reviews Into Scenario Rules

Review labels are not automatically applied to scenarios. They are evidence
for candidate rules. Use `scenario patch-preview` to inspect the concrete
scenario YAML change before editing or committing it.

- `unsafe_action` can become `autopsy.forbidden_actions`.
- `good_diagnostic` or `missed_diagnostic` can become
  `autopsy.expected_diagnostics`.
- `acceptable_mutation` can become `autopsy.allowed_mutations`.
- `retry_loop` and `premature_success` usually point to missing expected
  diagnostics, verifier checks, or scenario stop conditions.

Keep rules behavior-level where possible. Prefer `Pod/*` over exact ephemeral
Pod names when the problem is direct Pod deletion.

In the browser, save the review and then select `Preview scenario patch` from
the saved review's suggested-rules section. Hosted preview uses the same patch
builder as the CLI, stores the generated `scenario_patch_preview` artifact, and
never applies the patch. When a preview contains changes, `Download diff` uses
the backend `scenario-patch.diff` URL so the downloaded diff is the same durable
artifact clients can fetch from the API.
After applying the diff to the scenario catalog, `Validate rerun` queues the
same scenario/model/tool-server slice through the trigger API and stores a
`scenario_patch_validation.v1` record on the source run so the new run can be
compared against the reviewed source run.

Preview a scenario patch from a saved local review:

```bash
bench-cli scenario patch-preview \
  --scenario kubernetes/shared-configmap-trap \
  --review-file runs/<run-id>/run_review.json
```

The command prints a unified diff and does not modify `scenario.yaml`. Review
the diff, apply the change manually, rerun the scenario slice, and only keep the
rule when the new run produces better evidence.
