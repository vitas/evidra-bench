# Run Review Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Add the first human review vertical slice: `run_review.v1` schema, backend GET/PUT API, public review rendering in the browser, local TUI rendering, and user documentation.

**Architecture:** Store run reviews as the existing `bench_artifacts` row type `run_review` so the first implementation stays close to run evidence. Backend writes are authenticated and tenant-checked by loading the parent run before upserting the artifact. Reads follow run tenant scoping and hide non-public reviews from anonymous/public reads.

**Tech Stack:** Go service and tests, PostgreSQL artifact storage, React/Vite UI, Bubble Tea TUI, Markdown docs.

---

### Task 1: Shared Go Review Schema

**Files:**
- Create: `pkg/runreview/review.go`
- Create: `pkg/runreview/review_test.go`
- Modify: `pkg/artifact/catalog.go`

- [x] Add local and hosted review artifact constants:
  - `RunReviewFile = "run_review.json"`
  - `HostedRunReview = "run_review"`
- [x] Add `pkg/runreview` types for `Review`, `Reviewer`, `Label`, `EvidenceRef`, and `SuggestedRule`.
- [x] Add `NormalizeForRun(review Review, runID, scenarioID, defaultVisibility string) (Review, error)`.
- [x] Validate version, visibility, verdict, label kind, severity, parent run/scenario match, and high-severity note/evidence.
- [x] Test normalization fills run/scenario/default visibility.
- [x] Test anonymous/public visibility semantics by validating `visibility`.
- [x] Test high-severity labels require `note` and `evidence_snippet`.

### Task 2: Backend API

**Files:**
- Modify: `internal/benchsvc/handlers.go`
- Modify: `internal/benchsvc/handlers_artifacts.go`
- Modify: `internal/benchsvc/handlers_artifacts_test.go`
- Modify: `internal/benchsvc/handlers_test.go`

- [x] Add `GET /v1/bench/runs/{id}/review` under read middleware.
- [x] Add `PUT /v1/bench/runs/{id}/review` under authenticated middleware.
- [x] GET returns `404` for missing reviews.
- [x] GET returns `404` for anonymous reads of `visibility != "public"`.
- [x] PUT loads the run for the authenticated tenant, normalizes the review against that run, and stores artifact type `run_review`.
- [x] PUT defaults visibility to `public` when the writer tenant is the configured public tenant, otherwise `private`.
- [x] PUT rejects mismatched `run_id` or `scenario_id`.
- [x] Tests cover GET public, GET private hidden from anonymous, PUT normalized store, and PUT validation error.

### Task 3: Browser Review Tab

**Files:**
- Modify: `ui/src/pages/bench/RunDetail.tsx`
- Create: `ui/src/lib/runReview.mts`
- Create: `ui/src/lib/runReview.test.mts`

- [x] Add TypeScript review types and `normalizeRunReviewView`.
- [x] Add `review` tab to `RunDetail`.
- [x] Fetch `/v1/bench/runs/{id}/review` when Review tab is selected.
- [x] Render no-review, loading, and error states.
- [x] Render verdict, visibility, reviewer, labels, notes, evidence snippets, evidence refs, and suggested rules.
- [x] Keep browser write controls out of this slice because browser auth is not settled.
- [x] Test review normalization and display helper behavior.

### Task 4: TUI Review Rendering

**Files:**
- Modify: `pkg/tui/artifacts.go`
- Modify: `pkg/tui/artifacts_test.go`
- Modify: `pkg/tui/app_view.go`

- [x] Load local `run_review.json` into `RunArtifacts`.
- [x] Add `review` artifact tab.
- [x] Render review verdict, visibility, reviewer, labels, notes, evidence snippets, and suggested rules.
- [x] Include review availability in the artifact summary.
- [x] Test loading and rendering a local review artifact.

### Task 5: Documentation

**Files:**
- Create: `docs/contracts/RUN_REVIEW_V1.md`
- Create: `docs/guides/human-review.md`
- Modify: `docs/BENCH_API_REFERENCE.md`
- Modify: `docs/LAB_TUI_GUIDE.md`
- Modify: `docs/README.md`

- [x] Document schema, verdicts, labels, severity, visibility, evidence refs, and suggested rules.
- [x] Document browser public read behavior and backend-auth write behavior.
- [x] Document TUI local `run_review.json` rendering.
- [x] Link the new docs from the docs home.

### Task 6: Verification

**Files:**
- No source changes.

- [x] Run `go test ./pkg/runreview ./internal/benchsvc ./pkg/tui -count=1`.
- [x] Run `cd ui && node --test src/lib/runReview.test.mts src/lib/benchApi.test.mts`.
- [x] Run `make lint`.
- [x] Run `go test ./... -count=1`.
- [x] Run `cd ui && node --test src/lib/*.test.mts && npm run build`.
- [x] Commit with DCO sign-off.
