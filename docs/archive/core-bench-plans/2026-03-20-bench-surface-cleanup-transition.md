# Bench Surface Cleanup Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove deprecated benchmark codepaths and dead placeholder surfaces, keep one supported `/v1/bench/*` contract, and reshape the surviving bench code into a request-scoped service/repository boundary that can later transition into a fuller domain extraction.

**Architecture:** Execute the cleanup in three phases. First, replace the current process-global `benchsvc.PgStore` pattern with a request-scoped `benchsvc.Service` over a tenant-agnostic PostgreSQL repository so authenticated bench traffic becomes truly tenant-aware and run/artifact writes become atomic. Second, delete the legacy `/v1/benchmark/*` stack, old benchmark store/client code, dead exported bench methods, and stub-only endpoints/UI dependencies so the repo has one benchmark model and one HTTP family. Third, reconcile docs, OpenAPI, and validation scripts around the reduced contract, and document the new `service -> repository` seam as the transition point toward a later extraction to `internal/bench` or a dedicated API binary if that is still wanted.

**Tech Stack:** Go, `net/http`, pgx/PostgreSQL, React/Vite, OpenAPI YAML, Bash doc-guard scripts

---

### Task 1: Introduce a request-scoped bench service seam

**Files:**
- Create: `internal/benchsvc/service.go`
- Create: `internal/benchsvc/service_test.go`
- Modify: `internal/benchsvc/store.go`
- Modify: `internal/benchsvc/queries.go`
- Modify: `internal/benchsvc/import.go`
- Modify: `pkg/bench/types.go`

**Step 1: Write the failing service tests**

Add focused tests that lock the intended boundary:
- `Service.ListRuns(ctx, tenantID, filters)` uses the tenant passed by the caller, not a tenant captured at construction time
- `Service.IngestRun(...)` stores the run row and its artifacts atomically
- public leaderboard/scenario reads require an explicit public tenant configuration instead of silently reusing the authenticated/default tenant
- unsupported exported methods are removed from the supported bench contract

Example test skeleton:

```go
func TestServiceListRuns_UsesProvidedTenant(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, ServiceConfig{PublicTenant: "bench-public"})

	_, _, _ = svc.ListRuns(context.Background(), "tenant-b", bench.RunFilters{})

	if repo.lastTenant != "tenant-b" {
		t.Fatalf("tenant = %q, want tenant-b", repo.lastTenant)
	}
}

func TestServiceIngestRun_IsAtomic(t *testing.T) {
	repo := &fakeRepo{failStoreArtifact: true}
	svc := NewService(repo, ServiceConfig{})

	err := svc.IngestRun(context.Background(), "tenant-a", ingestRunRequest{
		RunRecord: bench.RunRecord{ID: "run-1", ScenarioID: "s1", Model: "m1"},
		Transcript: "hello",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if repo.insertedRun {
		t.Fatal("run insert should have been rolled back")
	}
}
```

**Step 2: Run the service tests to verify they fail**

Run:

```bash
go test ./internal/benchsvc -run 'TestService' -v
```

Expected: FAIL because `Service` does not exist and the current repository captures a single tenant at process startup.

**Step 3: Write minimal implementation**

Implement:
- `type Service struct { repo Repository; cfg ServiceConfig }`
- a tenant-agnostic repository interface used only inside `internal/benchsvc`
- service methods that always accept `tenantID string` for authenticated reads/writes
- one explicit `PublicTenant` config for public leaderboard/scenario routes; return `ErrPublicTenantUnavailable` when it is unset
- atomic ingest orchestration so the run row and all artifacts succeed or fail together

While doing this, trim `pkg/bench/types.go` down to supported operations only. Remove dead members from the exported contract:
- `CompareRuns`
- `ModelMatrix`
- `SignalSummary`
- `Regressions`
- `FailureAnalysis`
- dead exported result types used only by those methods

Do not add a new microservice here. The point of this task is to create the service/repository seam that a later extraction can build on.

**Step 4: Run the service tests to verify they pass**

Run:

```bash
go test ./internal/benchsvc -run 'TestService' -v
```

Expected: PASS

**Step 5: Run package verification**

Run:

```bash
go test ./pkg/bench ./internal/benchsvc -v
```

Expected: PASS

**Step 6: Commit**

```bash
git add internal/benchsvc/service.go internal/benchsvc/service_test.go internal/benchsvc/store.go internal/benchsvc/queries.go internal/benchsvc/import.go pkg/bench/types.go
git commit -m "refactor: add request-scoped bench service boundary"
```

---

### Task 2: Rewire HTTP handlers onto the new service and remove unsupported live routes

**Files:**
- Modify: `internal/benchsvc/handlers.go`
- Create: `internal/benchsvc/handlers_test.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/router_test.go`
- Modify: `cmd/evidra-api/main.go`

**Step 1: Write the failing handler and router tests**

Add tests for the supported live surface:
- authenticated `/v1/bench/runs`, `/v1/bench/runs/{id}`, `/v1/bench/runs/{id}/transcript`, `/v1/bench/runs/{id}/tool-calls`, `/v1/bench/runs/{id}/timeline`, `/v1/bench/runs/{id}/scorecard`, `/v1/bench/stats`, `/v1/bench/catalog`
- public `/v1/bench/leaderboard` and `/v1/bench/scenarios` use only the explicit public tenant
- `/v1/bench/signals` is no longer registered
- handler code reads the tenant from request context instead of from a store field

Example assertions:

```go
func TestRouter_DoesNotRegisterLegacyBenchSignals(t *testing.T) {
	router := NewRouter(RouterConfig{APIKey: "test", DefaultTenant: "t1", BenchService: fakeBenchService{}})
	req := httptest.NewRequest(http.MethodGet, "/v1/bench/signals", nil)
	req.Header.Set("Authorization", "Bearer test")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
```

**Step 2: Run the handler/router tests to verify they fail**

Run:

```bash
go test ./internal/api ./internal/benchsvc -run 'TestRouter|TestHandle' -v
```

Expected: FAIL because the current router still wires a store directly and still exposes `/v1/bench/signals`.

**Step 3: Write minimal implementation**

Implement:
- handlers that call `auth.TenantID(r.Context())` for authenticated bench routes
- public handlers that call service methods using the configured public tenant only
- router/config wiring that passes a service instead of a store
- removal of `/v1/bench/signals`

In `cmd/evidra-api/main.go`, stop constructing a tenant-bound `benchsvc.NewPgStore(pool, defaultTenant)` for all bench traffic. Build:
- a tenant-agnostic repository
- a bench service with `PublicTenant` config

If there is no configured public tenant, public bench endpoints should return a clear `503`/`404` style error instead of leaking authenticated/default data.

**Step 4: Run the handler/router tests to verify they pass**

Run:

```bash
go test ./internal/api ./internal/benchsvc -run 'TestRouter|TestHandle' -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/benchsvc/handlers.go internal/benchsvc/handlers_test.go internal/api/router.go internal/api/router_test.go cmd/evidra-api/main.go
git commit -m "refactor: route bench handlers through request-scoped service"
```

---

### Task 3: Delete the legacy `/v1/benchmark/*` stack and other dead benchmark code

**Files:**
- Delete: `internal/api/benchmark_handler.go`
- Delete: `internal/api/benchmark_handler_test.go`
- Delete: `internal/store/benchmarks.go`
- Delete: `internal/store/benchmarks_test.go`
- Delete: `cmd/evidra/benchmark.go`
- Delete: `cmd/evidra/benchmark_test.go`
- Modify: `internal/api/router.go`
- Modify: `cmd/evidra-api/main.go`
- Modify: `pkg/client/client.go`
- Modify: `pkg/client/client_test.go`
- Modify: `internal/db/db_test.go`
- Create: `internal/db/migrations/007_drop_legacy_benchmark_tables.up.sql`
- Create: `tests/test_no_legacy_benchmark_surface.sh`

**Step 1: Write the failing regression guard**

Create a shell guard that fails while the old surface still exists in active code/docs:

```bash
#!/usr/bin/env bash
set -euo pipefail

rg -n '/v1/benchmark/' cmd/evidra-api internal/api pkg/client docs/api-reference.md cmd/evidra-api/static/openapi.yaml ui/public/openapi.yaml \
  && { echo "legacy benchmark route still present" >&2; exit 1; }

test ! -e cmd/evidra/benchmark.go || { echo "dead benchmark cli stub still present" >&2; exit 1; }
```

Keep the search scoped to active code and docs. Do not fail on archived plan files.

**Step 2: Run the guard to verify it fails**

Run:

```bash
bash tests/test_no_legacy_benchmark_surface.sh
```

Expected: FAIL because the legacy API, client helpers, and dead CLI stub still exist.

**Step 3: Write minimal implementation**

Remove:
- the legacy router wiring for `/v1/benchmark/run`, `/v1/benchmark/runs`, `/v1/benchmark/compare`
- the old `BenchmarkStore` persistence/config plumbing
- old client request/response types and benchmark client methods in `pkg/client`
- the dead experimental `cmd/evidra benchmark` stub and its nonexistent roadmap reference

Database cleanup:
- add a new migration `007_drop_legacy_benchmark_tables.up.sql` that drops `benchmark_results` and `benchmark_runs`
- keep historical migration `003_benchmark_runs.up.sql` in repo history; do not renumber old migrations
- update `internal/db/db_test.go` to include both `006_bench_tables.up.sql` and the new `007_drop_legacy_benchmark_tables.up.sql`

**Step 4: Run the focused verification**

Run:

```bash
bash tests/test_no_legacy_benchmark_surface.sh
go test ./cmd/evidra ./cmd/evidra-api ./pkg/client ./internal/api ./internal/store ./internal/db -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/api/router.go cmd/evidra-api/main.go pkg/client/client.go pkg/client/client_test.go internal/db/db_test.go internal/db/migrations/007_drop_legacy_benchmark_tables.up.sql tests/test_no_legacy_benchmark_surface.sh
git add -u internal/api internal/store cmd/evidra
git commit -m "refactor: remove legacy benchmark surface"
```

---

### Task 4: Remove stub-only bench analytics/UI dependencies and make the surviving contract explicit

**Files:**
- Modify: `internal/benchsvc/handlers.go`
- Modify: `ui/src/pages/bench/BenchDashboard.tsx`
- Modify: `docs/api-reference.md`
- Modify: `cmd/evidra-api/static/openapi.yaml`
- Modify: `ui/public/openapi.yaml`
- Create: `internal/api/openapi_bench_docs_test.go`

**Step 1: Write the failing route/doc tests**

Add an OpenAPI test similar to `internal/api/openapi_ingest_docs_test.go` that asserts:
- `/v1/bench/runs/{id}/scorecard` exists in the OpenAPI spec
- `/v1/bench/signals` does not exist
- `/v1/benchmark/*` paths do not exist

Example shape:

```go
func TestOpenAPIBenchRoutesDocumentSupportedSurface(t *testing.T) {
	spec := loadOpenAPISpec(t)
	assertPathExists(t, spec, "/v1/bench/runs/{id}/scorecard")
	assertPathMissing(t, spec, "/v1/bench/signals")
	assertPathMissing(t, spec, "/v1/benchmark/run")
}
```

**Step 2: Run the focused tests to verify they fail**

Run:

```bash
go test ./internal/api -run 'TestOpenAPIBenchRoutesDocumentSupportedSurface|TestOpenAPIIngestRoutesDocumentContracts' -v
```

Expected: FAIL because the checked-in OpenAPI files still contain `/v1/benchmark/*` and omit `/v1/bench/runs/{id}/scorecard`.

**Step 3: Write minimal implementation**

Remove the fake signals surface completely:
- delete `/v1/bench/signals` from handlers and docs
- remove `SignalAggregation`-driven sections from `BenchDashboard.tsx`
- stop fetching `/v1/bench/signals`

Keep only metrics that the backend actually computes today:
- stats
- recent runs
- filtered run lists
- run detail artifacts
- per-run scorecard

Update both OpenAPI files and the markdown reference so the supported bench contract is:
- `/v1/bench/leaderboard`
- `/v1/bench/scenarios`
- `/v1/bench/runs`
- `/v1/bench/runs/batch`
- `/v1/bench/runs/{id}`
- `/v1/bench/runs/{id}/transcript`
- `/v1/bench/runs/{id}/tool-calls`
- `/v1/bench/runs/{id}/timeline`
- `/v1/bench/runs/{id}/scorecard`
- `/v1/bench/stats`
- `/v1/bench/catalog`

**Step 4: Run verification**

Run:

```bash
go test ./internal/api -run 'TestOpenAPIBenchRoutesDocumentSupportedSurface|TestOpenAPIIngestRoutesDocumentContracts' -v
npm --prefix ui run build
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/benchsvc/handlers.go ui/src/pages/bench/BenchDashboard.tsx docs/api-reference.md cmd/evidra-api/static/openapi.yaml ui/public/openapi.yaml internal/api/openapi_bench_docs_test.go
git commit -m "docs: align bench api with supported surface"
```

---

### Task 5: Repair docs, doc guards, and architecture wording around the new seam

**Files:**
- Modify: `README.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/guides/self-hosted-setup.md`
- Modify: `docs/guides/acceptance-fixture-status.md`
- Modify: `docs/integrations/cli-reference.md`
- Modify: `tests/tests-index.md`
- Modify: `CONTRIBUTING.md`
- Modify: `tests/test_doc_trust_alignment.sh`
- Modify: `tests/test_public_claims.sh`
- Modify: `tests/test_supported_core_positioning.sh`
- Modify: `tests/test_mcp_registry_publication_guide.sh`

**Step 1: Run the current failing doc checks**

Run:

```bash
bash tests/test_doc_trust_alignment.sh
bash tests/test_public_claims.sh
bash tests/test_supported_core_positioning.sh
bash tests/test_mcp_registry_publication_guide.sh
```

Expected: FAIL on current `main`.

**Step 2: Write minimal documentation changes**

Fix the active docs instead of recreating dead placeholders:
- do not restore `docs/ROAD_MAP.md` just to satisfy stale tests
- retarget tests and references to living docs (`README.md`, `docs/ARCHITECTURE.md`, `docs/guides/self-hosted-setup.md`)
- fix broken relative links in:
  - `docs/guides/acceptance-fixture-status.md`
  - `docs/integrations/cli-reference.md`
  - `tests/tests-index.md`
- add the missing MCP registry publication guide link to `README.md`
- align `CONTRIBUTING.md` with the Go version in `go.mod`
- remove the nonexistent `cmd/bench-api/` claim from `docs/ARCHITECTURE.md`

Replace that claim with explicit transition wording, e.g.:
- current bench architecture is `HTTP handlers -> benchsvc.Service -> PostgreSQL repository`
- this is the extraction seam for a later standalone bench domain/API if still needed

**Step 3: Add a simple broken-link verification script**

Add or extend a doc guard so the repo checks local markdown links in active docs. Prefer a small Bash script under `tests/` that validates local relative links in:
- `README.md`
- `docs/**/*.md`
- `tests/tests-index.md`

Keep it scoped to active docs and exclude archived plan files.

**Step 4: Run the doc verification**

Run:

```bash
bash tests/test_doc_trust_alignment.sh
bash tests/test_public_claims.sh
bash tests/test_supported_core_positioning.sh
bash tests/test_mcp_registry_publication_guide.sh
bash scripts/check-doc-commands.sh
```

Expected: PASS

If a new broken-link script is added, run it here and expect PASS.

**Step 5: Commit**

```bash
git add README.md docs/ARCHITECTURE.md docs/guides/self-hosted-setup.md docs/guides/acceptance-fixture-status.md docs/integrations/cli-reference.md tests/tests-index.md CONTRIBUTING.md tests/test_doc_trust_alignment.sh tests/test_public_claims.sh tests/test_supported_core_positioning.sh tests/test_mcp_registry_publication_guide.sh
git commit -m "docs: remove dead benchmark references and fix active links"
```

---

### Task 6: Run the full repo verification and close the cleanup batch

**Files:**
- Modify only if verification exposes real cleanup regressions

**Step 1: Run Go test coverage across the repo**

Run:

```bash
go test ./... -count=1
```

Expected: PASS

**Step 2: Run lint**

Run:

```bash
golangci-lint run
```

Expected: PASS

If lint fails, fix only real issues introduced or exposed by this cleanup batch. Do not leave ignored `errcheck` regressions behind.

**Step 3: Run the active doc and contract checks**

Run:

```bash
bash tests/test_module_path_refs.sh
bash tests/test_mode_labels_docs.sh
bash tests/test_hosted_architecture_docs.sh
bash tests/test_external_ingest_docs.sh
bash tests/test_protocol_docs.sh
bash tests/test_no_legacy_benchmark_surface.sh
bash scripts/check-doc-commands.sh
```

Expected: PASS

**Step 4: Run UI verification**

Run:

```bash
npm --prefix ui run build
git diff --check
```

Expected: PASS

**Step 5: Commit final follow-up fixes if needed**

```bash
git add -A
git commit -m "chore: finish benchmark surface cleanup verification"
```

Only create this commit if verification required additional fixes after the earlier task commits.

---

## Transition Notes

This plan intentionally stops short of a full approach-3 extraction. The required transition artifact after this cleanup is:
- `benchsvc.Service` owns request-scoped orchestration and tenant selection
- the PostgreSQL implementation is a tenant-agnostic repository behind that service
- HTTP handlers know only the service boundary

Once that shape exists, a later extraction can move the service and repository into a separate domain package or API binary without carrying the current legacy route/store/doc baggage forward.

