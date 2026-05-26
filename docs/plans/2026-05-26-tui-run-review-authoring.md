# TUI Run Review Authoring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add the next human review slice: author `run_review.v1` from the TUI artifact view, save it locally, and optionally upload it through the existing hosted review API.

**Architecture:** Keep the browser read-only until browser auth is settled. Use the existing TUI artifact bundle and `pkg/runreview` schema as the authoring source of truth. The editor builds one review label from a selected timeline step, writes `run_review.json`, refreshes the artifact tab, and can send the same JSON payload to `PUT /v1/bench/runs/{id}/review` when Bench API credentials are configured.

**Tech Stack:** Go, Bubble Tea TUI, local artifact files, hosted Bench API, Markdown docs.

---

### Task 1: Review Draft Builder

**Files:**
- Modify: `pkg/tui/artifacts.go`
- Create: `pkg/tui/review_editor.go`
- Create: `pkg/tui/review_editor_test.go`

- [x] Write a failing test that loading local artifacts records the run ID and scenario ID from `run.json`/directory name.
- [x] Write a failing test that a selected timeline step becomes a normalized `run_review.v1` draft with default verdict, visibility, note, evidence snippet, evidence ref, and suggested rule.
- [x] Implement run metadata loading and the draft builder.
- [x] Run `go test ./pkg/tui -run 'TestLoadRunArtifactsIncludesReviewRunMetadata|TestBuildReviewFromEditor' -count=1`.

### Task 2: Local Save And Hosted Upload

**Files:**
- Modify: `pkg/tui/review_editor.go`
- Modify: `pkg/tui/review_editor_test.go`

- [x] Write a failing test that saving a review writes pretty `run_review.json`.
- [x] Write a failing test that upload sends `PUT /v1/bench/runs/{id}/review` with a Bearer token and JSON body.
- [x] Implement local save and hosted upload helpers.
- [x] Run `go test ./pkg/tui -run 'TestSaveRunReview|TestUploadRunReview' -count=1`.

### Task 3: TUI Editor View

**Files:**
- Modify: `pkg/tui/app.go`
- Modify: `pkg/tui/app_update.go`
- Modify: `pkg/tui/app_view.go`
- Modify: `pkg/tui/app_test.go`

- [x] Write a failing test that pressing `r` from artifact view opens the review editor.
- [x] Write a failing test that `w` saves a local review and returns to the review artifact tab.
- [x] Implement editor state, key handling, and render output.
- [x] Run `go test ./pkg/tui -run 'TestReviewEditor' -count=1`.

### Task 4: Documentation

**Files:**
- Modify: `docs/guides/human-review.md`
- Modify: `docs/LAB_TUI_GUIDE.md`
- Modify: `docs/plans/2026-05-26-tui-run-review-authoring.md`

- [x] Document TUI review authoring keys, local save behavior, default evidence snippet/note, and optional upload.
- [x] Mark this implementation plan complete.

### Task 5: Verification

**Files:**
- No source changes.

- [x] Run `make lint`.
- [x] Run `go test ./... -count=1`.
- [x] Run `cd ui && node --test src/lib/*.test.mts && npm run build`.
- [x] Run hygiene scripts that apply to shipped changes.
- [x] Commit with DCO sign-off.
