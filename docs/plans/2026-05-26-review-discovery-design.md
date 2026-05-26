---
title: Review Discovery Design
type: design
status: active
date: 2026-05-26
tags:
  - bench
  - review
  - ui
  - api
---

# Review Discovery Design

## Goal

Make human reviews visible across the product instead of hiding them inside
individual run pages.

## Product Shape

The next layer is review discovery:

- run list and run detail responses include a compact `review_summary`
- the Runs table shows a human verdict chip when a review exists
- a Review Queue page lists runs that need human review, reviewed unsafe
  passes, and reviewed failures

This is still the same product in public and private deployments. Public users
see summaries only for public reviews on public runs. Authenticated tenant
reads see the tenant's private review summaries.

## API Shape

Add an optional `review_summary` object to `bench.RunRecord` JSON:

```json
{
  "review_summary": {
    "verdict": "unsafe_pass",
    "primary_label": "unsafe_action",
    "visibility": "public",
    "label_count": 1,
    "max_severity": "warning"
  }
}
```

The summary is derived from the existing `run_review` artifact. It does not add
a new database table in this slice. If review filtering or aggregate analytics
become expensive, reviews can become first-class rows later without changing
the external shape.

## Queue Rules

The first queue is client-side over the recent runs endpoint:

- `Needs review`: runs without `review_summary`
- `Unsafe passes`: passed runs with `review_summary.verdict = unsafe_pass`
- `Reviewed failures`: failed runs with any review summary

This keeps the slice small and still makes review work discoverable. Backend
filters for `has_review` and `review_verdict` can follow once summary demand is
proven.

## Implementation Plan

1. Add `bench.RunReviewSummary`.
2. Derive summaries in runs handlers from visible `run_review` artifacts.
3. Add backend tests for list/detail summary visibility.
4. Add UI types and formatting helpers.
5. Show review chips in the Runs table.
6. Add `/bench/reviews` queue page and nav item.
7. Update API/user docs.
8. Verify lint, tests, UI build, and hygiene scripts.
