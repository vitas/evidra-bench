---
title: Browser Review Authoring Design
type: plan
status: active
tags:
  - bench
  - review
  - ui
---

# Browser Review Authoring Design

## Goal

Let an authenticated reviewer create or replace a `run_review.v1` artifact from
the browser Run Detail `Review` tab. Discovery already exists; this slice
closes the loop by making the same product surface writable in private
deployments without adding a parallel API.

## Product Shape

The Review tab shows the existing saved review plus a compact editor. The
editor captures the first practical slice of review authoring:

- verdict
- visibility
- label kind
- severity
- reviewer display name
- reviewer note
- evidence snippet
- optional timeline step
- optional suggested rule target and pattern

There is no draft state. Saving replaces the current review for the run.

## Data Flow

The browser builds a `run_review.v1` payload and calls the existing
`PUT /v1/bench/runs/{id}/review` endpoint. The backend still owns validation,
tenant scoping, default identity fields, and artifact storage. The browser does
not embed static API keys; deployments that allow browser writes must provide
authenticated backend access at the deployment boundary.

After save, the Run Detail page updates the saved review and refreshes the run
record so `review_summary` stays consistent with the review queues.

## Evidence Prefill

When timeline data is available, the editor picks the first mutation-like step
as the default evidence step and copies its command, operation, or summary as
the evidence snippet. If timeline data is unavailable, the editor still works
with manual evidence text.

## Tests

- pure UI helper tests cover default draft creation and payload generation
- UI build proves the Review tab wiring compiles
- existing backend tests continue to cover validation and artifact writes

