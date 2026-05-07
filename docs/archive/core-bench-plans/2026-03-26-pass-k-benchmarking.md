# pass^k Benchmarking Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add pass^k reliability metric to the leaderboard — measures consistency across multiple trials, not just single-attempt pass rate.

**Architecture:** Server-side computation via a side CTE joined to the existing leaderboard aggregation. Existing metrics (pass_rate, avg_duration, avg_cost, total_cost) keep their current run-weighted semantics. pass^k is computed separately from per-scenario pass rates. k defaults to 3, configurable via `?k=N` query param. Only scenarios with >= k trials contribute to pass^k.

**Tech Stack:** Go (SQL), PostgreSQL, React/TypeScript

**Key constraint:** The existing leaderboard query must not change behavior for current consumers. pass^k fields are purely additive.

---

### Task 1: Add pass^k fields to LeaderboardEntry type

**Files:**
- Modify: `pkg/bench/types.go:10-18`
- Test: `pkg/bench/types_test.go`

**Step 1: Add fields to LeaderboardEntry**

```go
type LeaderboardEntry struct {
	Model               string  `json:"model"`
	Scenarios           int     `json:"scenarios"`
	Runs                int     `json:"runs"`
	PassRate            float64 `json:"pass_rate"`
	AvgDuration         float64 `json:"avg_duration"`
	AvgCost             float64 `json:"avg_cost"`
	TotalCost           float64 `json:"total_cost"`
	PassK               float64 `json:"pass_k"`               // pass^k reliability (0-100)
	PassKTrials         int     `json:"pass_k_trials"`         // k value used
	SufficientScenarios int     `json:"sufficient_scenarios"`  // scenarios with >= k trials
}
```

**Step 2: Write test, run, verify, commit**

```bash
gofmt -w .
go test ./pkg/bench/ -count=1
git add pkg/bench/types.go pkg/bench/types_test.go
git commit -s -m "feat(bench): add PassK reliability fields to LeaderboardEntry"
```

---

### Task 2: Compute pass^k in leaderboard SQL — additive side CTE

**Files:**
- Modify: `internal/benchsvc/leaderboard.go`
- Modify: `internal/benchsvc/service.go` (Leaderboard signature adds k param)
- Modify: `internal/benchsvc/handlers.go` (parse ?k= query param)
- Create: `internal/benchsvc/leaderboard_integration_test.go`

**Critical: Preserve existing aggregation semantics.**

The current query computes run-weighted aggregates directly from bench_runs.
The new query adds a side CTE for pass^k, joined to the existing aggregation.

**Step 1: Update Leaderboard signature**

Repository interface and Service method gain a `k int` parameter:

```go
// Repository
Leaderboard(ctx context.Context, tenantID string, evidenceMode string, k int) ([]bench.LeaderboardEntry, error)

// Service
func (s *Service) Leaderboard(ctx context.Context, evidenceMode string, k int) ([]bench.LeaderboardEntry, error) {
	if s.cfg.PublicTenant == "" {
		return nil, ErrPublicTenantUnavailable
	}
	return s.repo.Leaderboard(ctx, s.cfg.PublicTenant, evidenceMode, k)
}
```

**Step 2: Rewrite leaderboard.go**

The key: existing aggregation stays as-is (run-weighted). pass^k comes from a
separate CTE that computes per-scenario pass rates and applies POWER(rate, k).

```go
func (s *PgStore) Leaderboard(ctx context.Context, tenantID string, evidenceMode string, k int) ([]bench.LeaderboardEntry, error) {
	if k < 1 {
		k = 3
	}

	// Build the evidence mode WHERE clause using the shared alias logic.
	// This must use evidenceModeClause(), NOT a hardcoded = $N.
	argN := 2
	modeClause := ""
	var modeArgs []any
	if evidenceMode != "" {
		clause, arg := evidenceModeClause(argN, evidenceMode)
		modeClause = " AND " + clause
		modeArgs = append(modeArgs, arg)
		argN++
	}

	kArgN := argN
	args := []any{tenantID}
	args = append(args, modeArgs...)
	args = append(args, k)

	query := fmt.Sprintf(`
		-- Existing run-weighted aggregation (unchanged semantics).
		WITH run_agg AS (
			SELECT model,
				COUNT(DISTINCT scenario_id) AS scenarios,
				COUNT(*) AS runs,
				100.0 * SUM(CASE WHEN passed THEN 1 ELSE 0 END) / NULLIF(COUNT(*), 0) AS pass_rate,
				AVG(duration_seconds) AS avg_duration,
				AVG(estimated_cost_usd) AS avg_cost,
				SUM(estimated_cost_usd) AS total_cost
			FROM bench_runs
			WHERE tenant_id = $1 AND archived_at IS NULL%s
			GROUP BY model
		),
		-- Side CTE: per-scenario pass rates for pass^k.
		per_scenario AS (
			SELECT model, scenario_id,
				COUNT(*) AS trials,
				AVG(CASE WHEN passed THEN 1.0 ELSE 0.0 END) AS pass_rate
			FROM bench_runs
			WHERE tenant_id = $1 AND archived_at IS NULL%s
			GROUP BY model, scenario_id
		),
		pass_k_agg AS (
			SELECT model,
				COALESCE(100.0 * AVG(POWER(pass_rate, $%d)), 0) AS pass_k,
				COUNT(*)::int AS sufficient_scenarios
			FROM per_scenario
			WHERE trials >= $%d
			GROUP BY model
		)
		SELECT r.model, r.scenarios, r.runs, r.pass_rate,
			r.avg_duration, r.avg_cost, r.total_cost,
			COALESCE(p.pass_k, 0), $%d, COALESCE(p.sufficient_scenarios, 0)
		FROM run_agg r
		LEFT JOIN pass_k_agg p ON p.model = r.model
		ORDER BY r.pass_rate DESC, r.model
	`, modeClause, modeClause, kArgN, kArgN, kArgN)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("bench.Leaderboard: %w", err)
	}
	defer rows.Close()

	entries, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (bench.LeaderboardEntry, error) {
		var e bench.LeaderboardEntry
		err := row.Scan(&e.Model, &e.Scenarios, &e.Runs, &e.PassRate,
			&e.AvgDuration, &e.AvgCost, &e.TotalCost,
			&e.PassK, &e.PassKTrials, &e.SufficientScenarios)
		return e, err
	})
	if err != nil {
		return nil, fmt.Errorf("bench.Leaderboard: collect: %w", err)
	}
	return entries, nil
}
```

**Step 3: Parse ?k= in handleLeaderboard**

```go
k := 3
if kStr := r.URL.Query().Get("k"); kStr != "" {
	if kVal, err := strconv.Atoi(kStr); err == nil && kVal >= 1 && kVal <= 10 {
		k = kVal
	}
}
entries, err := svc.Leaderboard(r.Context(), mode, k)
```

**Step 4: Update all mock repos**

Update Leaderboard signature in `handlerRepo`, `fakeRepo`, `routerBenchRepo`.

**Step 5: Write integration test**

Create `internal/benchsvc/leaderboard_integration_test.go` with `//go:build integration`:

```go
func TestLeaderboard_PassKMath(t *testing.T) {
	// Seed 2 scenarios × 3 runs each for one model.
	// Scenario A: 3/3 pass → pass_rate=1.0, p^3=1.0
	// Scenario B: 1/3 pass → pass_rate=0.33, p^3=0.036
	// Expected pass^3 = avg(1.0, 0.036) * 100 = 51.8
	// Expected pass_rate = 4/6 * 100 = 66.7 (run-weighted, NOT scenario-weighted)
}
```

This test validates that pass_rate stays run-weighted while pass^k uses per-scenario rates.

**Step 6: gofmt, test, lint, commit**

```bash
gofmt -w .
go test ./internal/benchsvc/ -count=1
go test ./internal/api/ -count=1
make lint
git add internal/benchsvc/leaderboard.go internal/benchsvc/service.go \
  internal/benchsvc/handlers.go internal/benchsvc/handlers_test.go \
  internal/benchsvc/service_test.go internal/api/router_test.go \
  internal/benchsvc/leaderboard_integration_test.go
git commit -s -m "feat(bench): add pass^k to leaderboard via side CTE, preserve run-weighted metrics"
```

---

### Task 3: Add pass^k column to evidra leaderboard UI

**Files:**
- Modify: `ui/src/pages/bench/BenchLeaderboard.tsx`

**Step 1: Update both interfaces — LeaderboardEntry AND ModelStats**

The component uses a `ModelStats` shape projected from the API response.
Both must be updated:

```typescript
// API response shape
interface LeaderboardEntry {
  // ... existing fields ...
  pass_k: number;
  pass_k_trials: number;
  sufficient_scenarios: number;
}

// Projected shape for rendering
interface ModelStats {
  // ... existing fields ...
  passK: number;
  passKTrials: number;
  sufficientScenarios: number;
}
```

**Step 2: Update the projection** (where LeaderboardEntry → ModelStats mapping happens)

Add to the mapping:
```typescript
passK: entry.pass_k,
passKTrials: entry.pass_k_trials,
sufficientScenarios: entry.sufficient_scenarios,
```

**Step 3: Add column to table and color-code**

```tsx
<th>Reliability (pass^k)</th>
// ...
<td>
  <span className={`font-semibold ${
    m.passK >= 70 ? "text-green-400" :
    m.passK >= 40 ? "text-accent" :
    "text-red-400"
  }`}>
    {m.passK.toFixed(1)}%
  </span>
  <span className="text-[0.65rem] text-fg-muted ml-1">
    k={m.passKTrials}, {m.sufficientScenarios}/{m.scenarios} qualifying
  </span>
</td>
```

**Step 4: Build, verify, commit**

```bash
cd ui && npm run build
git add ui/src/pages/bench/BenchLeaderboard.tsx
git commit -s -m "feat(ui): add pass^k reliability column to leaderboard"
```

---

### Task 4: Defer — evidra-bench leaderboard migration

**Skipped for now.** The evidra-bench leaderboard (`ui/src/pages/bench/Leaderboard.tsx`)
computes stats client-side from `GET /v1/bench/runs?limit=1000`. Migrating it to use
`GET /v1/bench/leaderboard` is a separate task with its own scope (removes client
aggregation, adds evidence mode filtering, handles pagination properly).

Tracked in backlog as: "Migrate bench UI leaderboard to server-computed endpoint."

---

### Task 5: Update OpenAPI spec and docs

**Files:**
- Modify: `cmd/evidra-api/static/openapi.yaml`
- Copy: `cp cmd/evidra-api/static/openapi.yaml ui/public/openapi.yaml`
- Modify: `docs/api-reference.md`

**Step 1: Define LeaderboardEntry response schema in openapi.yaml**

The current leaderboard endpoint has only a bare `200` description with no schema.
Define the full response schema:

```yaml
/v1/bench/leaderboard:
  get:
    summary: Public model leaderboard
    tags: [Bench]
    parameters:
      - name: evidence_mode
        in: query
        schema:
          type: string
          enum: [all, none, evidra]
        description: Filter by evidence mode (evidra = all non-baseline runs)
      - name: k
        in: query
        schema:
          type: integer
          minimum: 1
          maximum: 10
          default: 3
        description: Number of trials for pass^k reliability metric
    responses:
      '200':
        description: Leaderboard entries
        content:
          application/json:
            schema:
              type: object
              properties:
                models:
                  type: array
                  items:
                    $ref: '#/components/schemas/LeaderboardEntry'
                evidence_mode:
                  type: string
```

Add `LeaderboardEntry` to components/schemas:

```yaml
LeaderboardEntry:
  type: object
  properties:
    model:
      type: string
    scenarios:
      type: integer
    runs:
      type: integer
    pass_rate:
      type: number
      description: Run-weighted pass rate (0-100)
    avg_duration:
      type: number
    avg_cost:
      type: number
    total_cost:
      type: number
    pass_k:
      type: number
      description: "pass^k reliability — probability all k trials pass per scenario, averaged across qualifying scenarios (0-100)"
    pass_k_trials:
      type: integer
      description: "k value used"
    sufficient_scenarios:
      type: integer
      description: "Scenarios with >= k trials"
```

**Step 2: Update api-reference.md leaderboard section with pass_k fields**

**Step 3: Copy, verify, commit**

```bash
cp cmd/evidra-api/static/openapi.yaml ui/public/openapi.yaml
make lint
git add cmd/evidra-api/static/openapi.yaml ui/public/openapi.yaml docs/api-reference.md
git commit -s -m "docs: add pass^k and LeaderboardEntry schema to OpenAPI and API reference"
```

---

## Dependency Graph

```
Task 1 (types) → Task 2 (SQL + handler + mocks + integration test)
                    → Task 3 (evidra UI)
                    → Task 5 (docs)

Task 4: deferred (separate backlog item)
```

Tasks 3 and 5 can run in parallel after Task 2.
