---
title: Review Filtering Design
type: plan
status: active
tags:
  - bench
  - review
  - api
---

# Review Filtering Design

## Goal

Move review discovery from client-side filtering over the latest runs to
backend-backed run filtering. The review page should ask the API for each queue
directly, so queue totals and contents are based on the caller-visible review
state rather than a fixed recent-run sample.

## API Shape

Extend `GET /v1/bench/runs` with review-aware query parameters:

- `review=reviewed|unreviewed`
- `review_verdict=<run_review.v1 verdict>`
- `review_severity=<label severity>`
- `review_visibility=public|private`
- `reviewer=<reviewer display name or type>`

These filters compose with existing run filters such as `passed`, `scenario`,
`model`, `tool_server`, and pagination. The queue requests become:

- needs review: `review=unreviewed`
- unsafe passes: `passed=true&review_verdict=unsafe_pass`
- reviewed failures: `passed=false&review=reviewed`

## Visibility Rule

Review filters use the caller-visible review state. Anonymous public reads only
match public reviews. Private reviews are invisible to anonymous reads, so a
run with only a private review is treated as unreviewed in that context.
Authenticated reads can match both public and private reviews for the
authenticated tenant.

## Implementation Notes

Keep `review_summary` as the response projection. The database filter should
join against the existing hosted `run_review` artifact, not introduce a new
review table in this slice. The UI should stop fetching one broad
`limit=200` run page and instead fetch the three queue slices independently.

## Tests

- handler tests prove review query parameters are parsed and private reviews do
  not satisfy anonymous reviewed filters
- query builder tests prove review filters generate review artifact predicates
- UI tests prove the review page API paths are backend-filtered

