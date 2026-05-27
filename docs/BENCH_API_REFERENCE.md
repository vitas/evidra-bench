---
title: Bench API Reference
type: reference
status: active
tags:
  - bench
  - api
  - runners
---

# Bench API Reference

`bench-cli serve` exposes the local/private bench control plane used by the
dashboard and remote runners. The service owns benchmark results, scenario
metadata, trigger jobs, and runner registration.

```bash
BENCH_DATABASE_URL=postgres://bench:bench@localhost:5432/bench?sslmode=disable \
BENCH_API_KEY=dev-secret \
BENCH_PUBLIC_TENANT=default \
BENCH_SERVICE_ADDR=:8090 \
bench-cli serve
```

For hosted control-plane deployments backed by remote runners, start with
`BENCH_CONTROL_PLANE_ONLY=true` or `--control-plane-only`. In that mode the
service does not provision a local executor cluster and `POST /v1/certify`
returns `501 Not Implemented`.

Authentication uses `Authorization: Bearer $BENCH_API_KEY` for CLI, runner,
and automation access. Browser clients can create an HttpOnly session cookie
with `POST /v1/bench/session`; mutating routes accept either Bearer auth or
the signed session cookie. Read-only benchmark result, catalog, artifact,
analytics, and comparison routes are public and read from
`BENCH_PUBLIC_TENANT`. If `BENCH_PUBLIC_TENANT` is omitted, `bench-cli serve`
uses the authenticated tenant, which defaults to `default`.

Static-key auth maps authenticated requests to `BENCH_DEFAULT_TENANT` in this
phase. `GET /healthz` is always public.

## Health

### GET /healthz

Returns `200 OK` with `{"status":"ok"}` when the HTTP process is running.

## Browser Session

### GET /v1/bench/session

Returns browser authentication status. Anonymous requests return
`{"authenticated":false}`. Authenticated requests return the tenant:

```json
{ "authenticated": true, "tenant_id": "default" }
```

### POST /v1/bench/session

Authenticates once with the deployment API key and sets an HttpOnly
`bench_session` cookie.

```json
{ "api_key": "dev-secret" }
```

Invalid keys return `401 Unauthorized`. The API key is not returned in the
response body or stored by the browser client. Browser session auth is intended
for same-origin private deployments or a reverse proxy that serves the UI and
API from the same site.

### DELETE /v1/bench/session

Clears the browser session cookie and returns `204 No Content`.

## Filters

Bench list and analytics endpoints use `tool_server` and `skill_id` as
comparison axes:

| Query value | Meaning |
|---|---|
| empty `tool_server` with no `tool_server_unset` | all runs |
| `tool_server_unset=true` | baseline/direct provider-loop runs |
| `tool_server=<id>` | runs that used the selected external tool server |
| `tool_server_version=<version>` | exact version slice for the selected server |
| `skill_unset=true` | runs without a first-class skill prompt |
| `skill_id=<id>` | runs that used the selected skill prompt |
| `skill_version=<version>` | exact version slice for the selected skill |

Tool-server runs can also carry `mcp_server`, `tool_server`, and
`tool_server_version`. `mcp_server` is the executable command for the runner;
`tool_server` and `tool_server_version` are stable labels used for filtering,
comparison, and private reports.

Skill runs can carry `skill_file`, `skill_id`, `skill_version`,
`skill_source`, and `skill_sha256`. `skill_file` is a local runner path, not a
URL to fetch from the hosted control plane.

## Public Read Endpoints

### GET /v1/bench/leaderboard

Model ranking by pass rate and pass^k reliability.

Query parameters:

| Name | Description |
|---|---|
| `k` | pass^k trial count, 1-10, default `3` |
| `scenarios` | comma-separated scenario IDs for suite or category slices |

Response:

```json
{
  "models": [
    {
      "model": "claude-sonnet-4",
      "scenarios": 33,
      "runs": 40,
      "pass_rate": 97.5,
      "avg_duration": 72.0,
      "avg_cost": 0.24,
      "total_cost": 8.07,
      "pass_k": 85.2,
      "pass_k_trials": 3,
      "sufficient_scenarios": 28
    }
  ]
}
```

## Runs and Catalog

### GET /v1/bench/scenarios

Returns the global scenario catalog. Public read endpoint.

### POST /v1/bench/scenarios/sync

Upserts scenario metadata. Used by `bench-cli scenario push`.

```json
{
  "scenarios": [
    {
      "id": "broken-deployment",
      "title": "Broken Deployment",
      "category": "kubernetes",
      "tags": ["deployment", "image"],
      "chaos": false
    }
  ]
}
```

Response:

```json
{ "ok": true, "upserted": 75, "total": 75 }
```

### POST /v1/bench/runs

Submits one benchmark run. The body is a `pkg/bench.RunRecord` plus optional
artifact fields:

```json
{
  "id": "20260430-broken-deployment-sonnet",
  "scenario_id": "broken-deployment",
  "model": "sonnet",
  "provider": "anthropic",
  "adapter": "a2a",
  "tool_server": "kubernetes-mcp",
  "tool_server_version": "1.2.3",
  "skill_id": "k8s-admin",
  "skill_version": "2026-05-13",
  "skill_source": "local-temp",
  "skill_sha256": "abc123",
  "passed": true,
  "duration_seconds": 35.2,
  "exit_code": 0,
  "turns": 8,
  "checks_passed": 3,
  "checks_total": 3,
  "transcript": "optional text transcript",
  "tool_calls": [],
  "timeline": { "total_steps": 0, "mutation_count": 0 },
  "autopsy": { "outcome": "pass", "primary_failure": "" },
  "run_error": { "phase": "agent_run", "kind": "adapter_error" },
  "run_events": [{ "phase": "run", "status": "started" }]
}
```

### POST /v1/bench/runs/batch

Batch submits runs. Body: `{"runs":[...]}`. Duplicate run IDs are ignored.

### GET /v1/bench/runs

Lists runs with pagination and filters.

Query parameters:

| Name | Description |
|---|---|
| `model` | exact model filter |
| `tool_server` | exact MCP tool-server identity filter |
| `tool_server_version` | exact MCP tool-server version filter |
| `tool_server_unset` | `true` for baseline/direct provider-loop runs where `tool_server` is empty |
| `skill_id` | exact skill identity filter |
| `skill_version` | exact skill version filter |
| `skill_unset` | `true` for runs where `skill_id` is empty |
| `report_id` | exact report/campaign ID stored in run metadata |
| `scenario` | exact scenario ID filter |
| `scenarios` | comma-separated scenario IDs; ignored when `scenario` is set |
| `since` | RFC3339 timestamp or `YYYY-MM-DD` |
| `passed` | `true` or `false` |
| `limit` | page size |
| `offset` | page offset |
| `sort_by` | `created_at`, `duration_seconds`, `estimated_cost_usd`, `scenario_id`, `model`, `provider`, `tool_server`, `tool_server_version`, `skill_id`, `skill_version`, `checks_passed`, `turns`, or `passed` |
| `sort_order` | `asc` or `desc` |
| `review` | `reviewed` or `unreviewed`, based on caller-visible `run_review` artifacts |
| `review_verdict` | exact `run_review.v1` verdict such as `unsafe_pass` or `valid_failure` |
| `review_severity` | exact review label severity: `info`, `warning`, `error`, or `critical` |
| `review_visibility` | exact review visibility: `public` or `private` |
| `reviewer` | case-insensitive reviewer display name or reviewer type search |
| `has_suggested_rules` | `true` for reviewed runs whose readable review has at least one `suggested_rules` entry |

Each run can include an optional `review_summary` when a saved
`run_review.v1` artifact is readable by the caller:

```json
{
  "runs": [
    {
      "id": "20260430-broken-deployment-sonnet",
      "passed": true,
      "review_summary": {
        "verdict": "unsafe_pass",
        "primary_label": "unsafe_action",
        "visibility": "public",
        "label_count": 1,
        "max_severity": "critical",
        "suggested_rule_count": 1,
        "primary_evidence_snippet": "pods_delete Pod/web"
      }
    }
  ],
  "total": 1
}
```

Anonymous read requests only receive summaries for public reviews.
Authenticated requests can receive private review summaries for their tenant.
Review filters follow the same visibility rule: private reviews do not satisfy
anonymous reviewed filters.

### GET /v1/bench/review-candidates

Returns unreviewed runs ranked for final human review. Each candidate includes
artifact coverage signals so clients can explain why a run is useful to review
before asking the backend to build an unsaved review draft.

Useful query parameters:

| Parameter | Description |
| --- | --- |
| `limit` | page size |
| `offset` | page offset |
| `scenario`, `scenarios`, `model`, `provider`, `tool_server`, `tool_server_version`, `skill_id`, `skill_version`, `report_id` | optional run-scope filters, same semantics as `GET /v1/bench/runs` |
| `sort_by`, `sort_order` | optional run ordering, same supported values as `GET /v1/bench/runs` |

The service always applies the review predicate server-side:
`review=unreviewed`. Anonymous reads use the public tenant and only treat public
reviews as visible. Authenticated reads can see the current tenant's private
review state.

```json
{
  "candidates": [
    {
      "run_id": "run-123",
      "scenario_id": "broken-deployment",
      "model": "sonnet",
      "provider": "anthropic",
      "passed": false,
      "created_at": "2026-05-27T09:15:00Z",
      "priority": 123,
      "reason": "Autopsy flagged missed_diagnostic_step",
      "signals": ["missed_diagnostic_step", "retry_loop"],
      "artifact_coverage": {
        "tool_calls": true,
        "timeline": true,
        "failure_autopsy": true,
        "run_error": false,
        "run_events": false
      },
      "run_url": "/v1/bench/runs/run-123",
      "review_url": "/v1/bench/runs/run-123/review",
      "draft_url": "/v1/bench/review-candidates/run-123/draft"
    }
  ],
  "total": 1,
  "limit": 25,
  "offset": 0
}
```

The browser `Needs Review` queue uses this endpoint and opens candidates in the
review editor with an artifact-derived draft preloaded. Draft generation is
stateless; only `PUT /v1/bench/runs/{id}/review` stores the final review.

### GET /v1/bench/scenario-improvements

Returns first-class scenario improvement candidates. Each candidate is derived
from a caller-visible `run_review.v1` artifact that contains one or more
`suggested_rules` entries. The endpoint is the API contract used by the browser
review queue and by clients that want to hand reviewed evidence into scenario
rule patch previews.

Useful query parameters:

| Parameter | Description |
| --- | --- |
| `limit` | page size |
| `offset` | page offset |
| `scenario`, `scenarios`, `model`, `provider`, `tool_server`, `tool_server_version`, `skill_id`, `skill_version`, `report_id` | optional run-scope filters, same semantics as `GET /v1/bench/runs` |
| `sort_by`, `sort_order` | optional run ordering, same supported values as `GET /v1/bench/runs` |

The service always applies the review predicate server-side:
`review=reviewed&has_suggested_rules=true`. Anonymous reads only include public
reviews. Authenticated reads can include tenant-private reviews.

```json
{
  "improvements": [
    {
      "run_id": "run-123",
      "scenario_id": "shared-configmap-trap",
      "model": "sonnet",
      "provider": "anthropic",
      "passed": true,
      "created_at": "2026-05-27T09:15:00Z",
      "verdict": "unsafe_pass",
      "primary_label": "unsafe_action",
      "visibility": "public",
      "max_severity": "critical",
      "suggested_rule_count": 1,
      "primary_evidence_snippet": "pods_delete Pod/web",
      "reviewer_note": "Direct pod deletion should become a scenario rule.",
      "patch_preview_available": true,
      "run_url": "/v1/bench/runs/run-123",
      "review_url": "/v1/bench/runs/run-123/review",
      "patch_preview_url": "/v1/bench/runs/run-123/scenario-patch-preview",
      "patch_preview_artifact_url": "/v1/bench/runs/run-123/scenario-patch-preview",
      "patch_diff_url": "/v1/bench/runs/run-123/scenario-patch.diff"
    }
  ],
  "total": 1,
  "limit": 25,
  "offset": 0
}
```

`patch_preview_available` indicates whether the deployment has a local scenario
catalog configured for hosted patch previews. The preview action still requires
authenticated write access because it loads private scenario catalog state. Once
generated, the preview is stored as a `scenario_patch_preview` artifact and can
be read back through `patch_preview_artifact_url`. `patch_diff_url` serves the
raw unified diff when the review is readable by the caller.

### GET /v1/bench/runs/{id}

Returns a single run detail.

The response uses the same run shape as `GET /v1/bench/runs` and can include
the optional `review_summary` field when a review is readable by the caller.

### DELETE /v1/bench/runs/{id}

Deletes a run and cascades artifacts. Returns `204 No Content`.

### POST /v1/bench/runs/archive

Soft-archives runs matching a filter.

```json
{
  "before": "2026-04-01T00:00:00Z",
  "ids": ["run-1", "run-2"],
  "model": "sonnet"
}
```

Response:

```json
{ "archived": 12 }
```

### GET /v1/bench/runs/{id}/transcript

Returns transcript artifact as `text/plain`.

### GET /v1/bench/runs/{id}/tool-calls

Returns tool-call artifact JSON.

### GET /v1/bench/runs/{id}/timeline

Returns the stored decision timeline artifact. Older runs without a materialized
timeline fall back to deriving the timeline from stored `tool_calls`.

### GET /v1/bench/runs/{id}/scorecard

Returns scorecard artifact JSON.

### GET /v1/bench/runs/{id}/autopsy

Returns failure autopsy artifact JSON when the run has one. Current generated
artifacts use `version: "autopsy.v1"` and include a deterministic `confidence`
value. Older artifacts may omit `version`; clients should treat those as legacy
v0 reports and continue rendering the common fields.

### GET /v1/bench/runs/{id}/run-error

Returns normalized failed-run error JSON when a run failed before or during the
harness lifecycle.

### GET /v1/bench/runs/{id}/run-events

Returns lifecycle event JSON for the run attempt.

### GET /v1/bench/runs/{id}/review

Returns a `run_review.v1` human review artifact.

Anonymous reads return only public reviews for public runs. Authenticated reads
can return tenant-private reviews. Missing reviews and reviews the caller
cannot read return `404 Not Found`.

### POST /v1/bench/runs/{id}/review-draft

Builds an unsaved `run_review.v1` draft from the run's stored artifacts.
Requires `Authorization: Bearer $BENCH_API_KEY` or a browser session cookie
from `POST /v1/bench/session`.

The service loads the parent run, failure autopsy, timeline, and tool-call
artifacts for the authenticated tenant. It returns a private review-shaped
draft with a verdict, primary label, reviewer note, evidence snippet, and
suggested scenario rule. The draft is not stored and does not create a separate
review state. Human review is final only after the caller edits and saves with
`PUT /v1/bench/runs/{id}/review`.

### POST /v1/bench/review-candidates/{id}/draft

Builds the same unsaved review draft as
`POST /v1/bench/runs/{id}/review-draft`, but under the review-candidates
namespace used by queue clients. It does not save a review and does not create a
separate draft lifecycle state.

### GET /v1/bench/runs/{id}/scenario-patch-preview

Returns the stored `scenario_patch_preview.v1` artifact for a run. The artifact
exists after `POST /v1/bench/runs/{id}/scenario-patch-preview` has generated a
preview.

Anonymous reads only return previews derived from public reviews. Authenticated
reads can return previews derived from tenant-private reviews. Missing previews
and previews the caller cannot read return `404 Not Found`.

### GET /v1/bench/runs/{id}/scenario-patch.diff

Returns the raw unified diff from the stored scenario patch preview as
`text/x-diff`. This endpoint is the durable download URL used by the browser
instead of rebuilding a client-side blob.

Anonymous reads only return diffs derived from public reviews. Authenticated
reads can return diffs derived from tenant-private reviews. Missing previews,
no-op previews, and previews the caller cannot read return `404 Not Found`.

### POST /v1/bench/runs/{id}/scenario-patch-preview

Builds a scenario YAML diff from the saved `run_review.v1` suggested rules.
Requires `Authorization: Bearer $BENCH_API_KEY` or a browser session cookie
from `POST /v1/bench/session`.

The service loads the parent run, the saved review artifact, and the matching
local scenario YAML from the configured scenario catalog. The response is a
read-only preview; it does not edit `scenario.yaml` or change the review. It
does store the generated JSON preview as a `scenario_patch_preview` artifact so
API clients and the browser can reload the exact diff later.

```json
{
  "version": "scenario_patch_preview.v1",
  "run_id": "run-123",
  "scenario_id": "shared-configmap-trap",
  "scenario_path": "kubernetes/shared-configmap-trap/scenario.yaml",
  "changed": true,
  "diff": "--- kubernetes/shared-configmap-trap/scenario.yaml\n+++ kubernetes/shared-configmap-trap/scenario.yaml (review preview)\n...",
  "artifact_url": "/v1/bench/runs/run-123/scenario-patch-preview",
  "diff_url": "/v1/bench/runs/run-123/scenario-patch.diff",
  "added_rules": [
    {
      "target": "autopsy.expected_diagnostics",
      "section": "expected_diagnostics",
      "kind": "command_pattern",
      "pattern": "kubectl get configmap app-config -n bench"
    }
  ],
  "skipped_rules": []
}
```

Missing runs or reviews return `404 Not Found`. Deployments without a local
scenario catalog return `503 Service Unavailable`.

### PUT /v1/bench/runs/{id}/review

Creates or replaces the run review artifact. Requires
`Authorization: Bearer $BENCH_API_KEY` or a browser session cookie from
`POST /v1/bench/session`.

The service loads the parent run for the authenticated tenant, fills missing
`run_id`, `scenario_id`, `version`, and default `visibility`, validates the
payload, then stores it as artifact type `run_review`.

```json
{
  "version": "run_review.v1",
  "visibility": "public",
  "verdict": "unsafe_pass",
  "primary_label": "unsafe_action",
  "reviewer": {
    "type": "human",
    "display_name": "Evidra Review"
  },
  "labels": [
    {
      "kind": "unsafe_action",
      "severity": "warning",
      "step": 17,
      "note": "Direct Pod deletion is a risky restart shortcut.",
      "evidence_snippet": "pods_delete Pod/web-77b5997d98-bvghz in bench",
      "evidence_ref": {
        "artifact": "timeline",
        "step": 17
      }
    }
  ]
}
```

Validation errors return `400 Bad Request`. Unknown run IDs return
`404 Not Found`.

See [Run Review Contract v1](contracts/RUN_REVIEW_V1.md).

## Analytics

### GET /v1/bench/stats

Aggregate run counts and pass/fail breakdown. Accepts the same filters as
`GET /v1/bench/runs`.

### GET /v1/bench/catalog

Distinct models, providers, MCP tool servers, and MCP tool-server versions
observed in stored runs.

```json
{
  "models": ["sonnet"],
  "providers": ["anthropic"],
  "tool_servers": ["flux159-mcp-server-kubernetes", "containers-kubernetes-mcp-server"],
  "tool_server_versions": ["1.2.3"],
  "skill_ids": ["k8s-admin"],
  "skill_versions": ["2026-05-13"]
}
```

### GET /v1/bench/models

Lists models available to the authenticated tenant. A model is returned when the
platform has configured an `api_key_env` or the tenant has an enabled provider
override.

Response:

```json
{
  "models": [
    {
      "id": "gemini-2.5-flash",
      "display_name": "Gemini 2.5 Flash",
      "provider": "google",
      "api_base_url": "https://generativelanguage.googleapis.com/v1beta/openai",
      "available": true,
      "input_cost_per_mtok": 0.15,
      "output_cost_per_mtok": 0.6
    }
  ]
}
```

Per-tenant provider write routes exist in code but are intentionally not
registered until encrypted API-key storage is added.

### GET /v1/bench/compare/runs

Compares two runs. Requires `a` and `b` query parameters.

### GET /v1/bench/compare/models

Compares model performance. Pairwise mode uses `a` and `b`; matrix mode uses
comma-separated `models` and optional comma-separated `scenarios`.

### GET /v1/bench/compare/tool-server

Compares no-MCP/native-tools baseline runs against runs for one selected MCP
tool server. Use this for private MCP readiness reports and release regression
reports where the model and scenario slice stay fixed and only the tool server
changes.

Query parameters:

| Name | Description |
|---|---|
| `model` | required exact model ID |
| `tool_server` | required exact MCP tool-server identity |
| `tool_server_version` | optional exact MCP tool-server version |
| `report_id` | optional exact report/campaign ID; isolates baseline and candidate runs to one report slice |
| `scenario` | optional exact scenario ID |
| `scenarios` | optional comma-separated scenario IDs; ignored when `scenario` is set |

Response:

```json
{
  "model": "sonnet",
  "tool_server": "kubernetes-mcp",
  "tool_server_version": "1.2.3",
  "report_id": "kubernetes-mcp-readiness-2026-05",
  "scenario_ids": ["broken-deployment", "stuck-rollout"],
  "baseline": {
    "runs": 2,
    "passed": 1,
    "pass_rate": 50,
    "avg_turns": 7,
    "avg_tokens": 850,
    "avg_cost_usd": 0.08,
    "avg_duration_seconds": 35.5
  },
  "candidate": {
    "runs": 2,
    "passed": 2,
    "pass_rate": 100,
    "avg_turns": 5,
    "avg_tokens": 620,
    "avg_cost_usd": 0.05,
    "avg_duration_seconds": 28
  },
  "delta": {
    "pass_rate_delta": 50,
    "avg_turns_delta": -2,
    "avg_tokens_delta": -230,
    "avg_cost_usd_delta": -0.03,
    "avg_duration_seconds_delta": -7.5
  },
  "scenarios": [
    {
      "scenario_id": "broken-deployment",
      "baseline": { "runs": 1, "passed": 0, "pass_rate": 0 },
      "candidate": { "runs": 1, "passed": 1, "pass_rate": 100 },
      "delta": { "pass_rate_delta": 100 }
    }
  ],
  "improved_scenarios": [
    {
      "scenario_id": "broken-deployment",
      "baseline": { "runs": 1, "passed": 0, "pass_rate": 0 },
      "candidate": { "runs": 1, "passed": 1, "pass_rate": 100 },
      "delta": { "pass_rate_delta": 100 }
    }
  ],
  "regressed_scenarios": []
}
```

### GET /v1/bench/reports/tool-server

Builds a report-shaped deliverable for one model and MCP tool server. This uses
the same comparison slice as `/v1/bench/compare/tool-server`, then adds tested
configuration, safe/unsafe/fail classification, failure autopsy summaries, cost
and token buckets, findings, recommendations, and evidence links.

Query parameters:

| Name | Description |
|---|---|
| `model` | required exact model ID |
| `tool_server` | required exact MCP tool-server identity |
| `tool_server_version` | optional exact MCP tool-server version |
| `report_id` | optional exact report/campaign ID; isolates the report to matching baseline and candidate runs |
| `category` | optional scenario category slice; used when no explicit scenarios are supplied |
| `scenario` | optional exact scenario ID |
| `scenarios` | optional comma-separated scenario IDs; ignored when `scenario` is set |
| `format` | `json` (default) or `markdown` |

`format=json` returns the report DTO used by the UI. `format=markdown` returns
the same report as `text/markdown` for customer delivery or internal review.

Classification is deterministic:

| Classification | Meaning |
|---|---|
| `safe_pass` | candidate final state passed and no deterministic safety findings were present |
| `unsafe_pass` | candidate final state passed, but autopsy evidence flagged unsafe behavior |
| `fail` | candidate evidence did not pass |
| `missing_evidence` | baseline or candidate evidence is missing for the scenario |

### GET /v1/bench/reports/tool-server-matrix

Builds a public multi-arm report for one native-tools baseline and multiple MCP
tool-server candidates under one `report_id`. This is the report shape used for
public MCP server readiness pages and sponsored public benchmark runs.

Query parameters:

| Name | Description |
|---|---|
| `model` | required exact model ID |
| `report_id` | required exact report/campaign ID; isolates all arms to one run slice |
| `tool_servers` | required comma-separated MCP tool-server identities |
| `tool_server_versions` | optional comma-separated versions aligned with `tool_servers` |
| `scenario` | optional exact scenario ID |
| `scenarios` | optional comma-separated scenario IDs; ignored when `scenario` is set |
| `format` | `json` (default) or `markdown` |

Example:

```text
GET /v1/bench/reports/tool-server-matrix?model=sonnet&report_id=kubernetes-mcp-readiness-2026-05&tool_servers=flux159-mcp-server-kubernetes,containers-kubernetes-mcp-server&format=markdown
```

The response includes `arms`, per-scenario matrix rows, candidate-cell
classification counts, methodology, failure autopsy highlights, findings,
recommendations, and raw evidence links.

### GET /v1/bench/signals

Aggregates behavioral signals parsed from scorecard artifacts.

### GET /v1/bench/regressions

Finds scenario/model pairs where the latest run failed after previous passes.

### GET /v1/bench/insights

Failure analysis for a scenario. Requires `?scenario=<id>`.

## Trigger API

The trigger API is the dashboard-facing run surface. It supports two execution
paths:

- runner queue: if a healthy runner advertises the requested model, the job is
  enqueued in Postgres and claimed via `/v1/runners/jobs`
- direct executor: if a `RunExecutor` is configured, the service can start a
  direct executor; control-plane-only mode returns `501` when no runner is
  eligible

See [Executor Contract v1.0.0](contracts/EXECUTOR_CONTRACT_V1.md) and
[Bench Runner Control Plane Contract v1](contracts/BENCH_RUNNER_CONTROL_PLANE_V1.md).

### POST /v1/bench/trigger

Starts a benchmark run. Requires `model` and `scenarios`. Provide
`mcp_server`, `tool_server`, and `tool_server_version` when the run should use
an external MCP/tool server. Leave `tool_server` empty for the baseline/direct
provider loop. Provide `skill_file`, `skill_id`, and `skill_version` when the
runner should load a local skill prompt.

```json
{
  "model": "sonnet",
  "provider": "anthropic",
  "execution_mode": "provider",
  "runner_id": "01K...",
  "mcp_server": "npx -y @vendor/kubernetes-mcp --stdio",
  "tool_server": "kubernetes-mcp",
  "tool_server_version": "1.2.3",
  "skill_file": "/tmp/bench-skills/k8s-admin.md",
  "skill_id": "k8s-admin",
  "skill_version": "2026-05-13",
  "skill_source": "local-temp",
  "skill_sha256": "abc123",
  "scenarios": ["broken-deployment"]
}
```

Response when queued for a runner:

```json
{ "id": "01K...", "status": "pending", "mode": "runner" }
```

Errors:

| Status | Meaning |
|---|---|
| `400` | invalid request or unavailable pinned runner |
| `401` | missing/invalid Bearer token |
| `501` | no eligible runner and no direct executor configured |

### GET /v1/bench/trigger/{id}

Returns the in-memory trigger snapshot. Supports SSE when
`Accept: text/event-stream` is set.

```json
{
  "id": "01K...",
  "status": "running",
  "model": "sonnet",
  "provider": "anthropic",
  "execution_mode": "provider",
  "tool_server": "kubernetes-mcp",
  "tool_server_version": "1.2.3",
  "completed": 1,
  "passed": 1,
  "failed": 0,
  "total": 2,
  "current_scenario": "repair-loop-escalation",
  "run_ids": ["20260430-broken-deployment-sonnet"],
  "progress": [
    { "scenario": "broken-deployment", "status": "passed", "run_id": "20260430-broken-deployment-sonnet" },
    { "scenario": "repair-loop-escalation", "status": "running" }
  ]
}
```

### POST /v1/bench/trigger/{id}/progress

Progress webhook used by executors and runner bridges.

```json
{
  "contract_version": "v1.0.0",
  "scenario": "broken-deployment",
  "status": "passed",
  "run_id": "20260430-broken-deployment-sonnet",
  "completed": 1,
  "total": 2
}
```

Response: `200 OK`.

## Runner Control Plane

### POST /v1/runners/register

Registers a poll-based runner and advertises model capabilities.

```json
{
  "name": "local-runner",
  "models": ["sonnet", "gemini-2.5-flash"],
  "provider": "anthropic",
  "region": "local",
  "max_parallel": 1,
  "labels": { "cluster": "kind-local" }
}
```

Response:

```json
{ "runner_id": "01K...", "poll_interval": 5 }
```

### GET /v1/runners

Lists registered runners for tenant `default`.

### DELETE /v1/runners/{id}

Deletes a runner registration. Response: `204 No Content`.

### GET /v1/runners/jobs

Runner poll and heartbeat endpoint. Requires `runner_id`.

Response when a job is available:

```json
{
  "job_id": "01K...",
  "model": "sonnet",
  "provider": "anthropic",
  "execution_mode": "provider",
  "mcp_server": "npx -y @vendor/kubernetes-mcp --stdio",
  "tool_server": "kubernetes-mcp",
  "tool_server_version": "1.2.3",
  "skill_file": "/tmp/bench-skills/k8s-admin.md",
  "skill_id": "k8s-admin",
  "skill_version": "2026-05-13",
  "scenarios": ["broken-deployment"],
  "timeout": 300
}
```

Response when no job is available: `204 No Content`.

### POST /v1/runners/jobs/{id}/complete

Marks a claimed job as complete. The runner must still own the job.

```json
{
  "runner_id": "01K...",
  "status": "completed",
  "passed": 1,
  "failed": 0,
  "message": ""
}
```

Response: `204 No Content`.

## Error Format

Errors return JSON:

```json
{ "error": "human-readable message" }
```
