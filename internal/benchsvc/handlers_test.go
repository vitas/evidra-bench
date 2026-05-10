package benchsvc

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"samebits.com/evidra-infra-bench/internal/auth"
	bench "samebits.com/evidra-infra-bench/pkg/bench"
)

// handlerRepo is an in-memory fake implementing Repository for handler tests.
// Each field holds canned return values; tests set them before making requests.
type handlerRepo struct {
	runs             []bench.RunRecord
	runsTotal        int
	runsErr          error
	run              *bench.RunRecord
	runErr           error
	stats            *bench.StatsResult
	statsErr         error
	catalog          *bench.RunCatalog
	catalogErr       error
	enabledModels    []EnabledModel
	enabledModelsErr error
	leaders          []bench.LeaderboardEntry
	leadersErr       error
	scenarios        []bench.ScenarioSummary
	scenErr          error
	artifact         []byte
	artCT            string
	artErr           error
	lastArtifactType string

	// delete / archive
	deleteErr         error
	deleteProviderErr error
	archiveCount      int
	archiveErr        error

	// analytics
	signals     *bench.SignalAggregation
	signalsErr  error
	regressions []bench.Regression
	regressErr  error
	insights    *bench.FailureInsights
	insightsErr error
	matrix      *bench.ModelMatrix
	matrixErr   error

	// runner
	registeredRunner *Runner
	foundRunner      *Runner
	runners          []Runner
	enqueuedJob      *BenchJob
	lastEnqueueCfg   JobConfig
	claimedJob       *BenchJob

	// capture
	lastTenant       string
	lastFilter       bench.RunFilters
	lastMode         string
	lastScenarios    []string
	lastModelID      string
	lastProviderCfg  TenantProviderConfig
	lastGlobalCfg    GlobalModelConfig
	modelProvider    *ModelProviderInfo
	modelProviderErr error
}

func (r *handlerRepo) ListRuns(_ context.Context, tenant string, f bench.RunFilters) ([]bench.RunRecord, int, error) {
	r.lastTenant = tenant
	r.lastFilter = f
	if r.runsErr != nil {
		return nil, 0, r.runsErr
	}
	filtered := filterRunsByEvidenceMode(r.runs, f.EvidenceMode)
	filtered = filterRunsByToolServer(filtered, f.ToolServer)
	if f.ScenarioID != "" {
		filtered = filterRunsByScenarioIDs(filtered, []string{f.ScenarioID})
		return filtered, len(filtered), nil
	}
	filtered = filterRunsByScenarioIDs(filtered, f.ScenarioIDs)
	return filtered, len(filtered), nil
}
func (r *handlerRepo) GetRun(_ context.Context, tenant, id string) (*bench.RunRecord, error) {
	r.lastTenant = tenant
	return r.run, r.runErr
}
func (r *handlerRepo) InsertRun(_ context.Context, _ string, _ bench.RunRecord) error { return nil }
func (r *handlerRepo) DeleteRun(_ context.Context, tenant, id string) error {
	r.lastTenant = tenant
	return r.deleteErr
}
func (r *handlerRepo) ArchiveRuns(_ context.Context, tenant string, _ ArchiveRequest) (int, error) {
	r.lastTenant = tenant
	return r.archiveCount, r.archiveErr
}
func (r *handlerRepo) InsertRunBatch(_ context.Context, _ string, _ []bench.RunRecord) (int, error) {
	return 0, nil
}
func (r *handlerRepo) FilteredStats(_ context.Context, tenant string, f bench.RunFilters) (*bench.StatsResult, error) {
	r.lastTenant = tenant
	r.lastFilter = f
	if r.statsErr != nil {
		return nil, r.statsErr
	}
	if r.stats != nil {
		return r.stats, nil
	}
	return aggregateStatsRuns(filterRunsByToolServer(filterRunsByEvidenceMode(r.runs, f.EvidenceMode), f.ToolServer)), nil
}
func (r *handlerRepo) Catalog(_ context.Context, tenant string) (*bench.RunCatalog, error) {
	r.lastTenant = tenant
	return r.catalog, r.catalogErr
}
func (r *handlerRepo) ListEnabledModels(_ context.Context, tenant string) ([]EnabledModel, error) {
	r.lastTenant = tenant
	return r.enabledModels, r.enabledModelsErr
}
func (r *handlerRepo) UpsertTenantProvider(_ context.Context, tenantID, modelID string, cfg TenantProviderConfig) error {
	r.lastTenant = tenantID
	r.lastModelID = modelID
	r.lastProviderCfg = cfg
	return nil
}
func (r *handlerRepo) DeleteTenantProvider(_ context.Context, tenantID, modelID string) error {
	r.lastTenant = tenantID
	r.lastModelID = modelID
	return r.deleteProviderErr
}
func (r *handlerRepo) UpdateGlobalModel(_ context.Context, modelID string, cfg GlobalModelConfig) error {
	r.lastModelID = modelID
	r.lastGlobalCfg = cfg
	return nil
}
func (r *handlerRepo) ResolveModelProvider(_ context.Context, modelID string) (*ModelProviderInfo, error) {
	if r.modelProvider == nil && r.modelProviderErr == nil {
		return nil, fmt.Errorf("model not found: %s", modelID)
	}
	return r.modelProvider, r.modelProviderErr
}
func (r *handlerRepo) Leaderboard(_ context.Context, tenant, mode string, _ int, scenarios []string) ([]bench.LeaderboardEntry, error) {
	r.lastTenant = tenant
	r.lastMode = mode
	r.lastScenarios = scenarios
	if r.leadersErr != nil {
		return nil, r.leadersErr
	}
	if r.leaders != nil {
		return r.leaders, nil
	}
	return aggregateLeaderboardRuns(filterRunsByScenarioIDs(filterRunsByEvidenceMode(r.runs, mode), scenarios)), nil
}
func (r *handlerRepo) ListScenarios(_ context.Context) ([]bench.ScenarioSummary, error) {
	return r.scenarios, r.scenErr
}
func (r *handlerRepo) StoreArtifact(_ context.Context, _, _, _ string, _ []byte) error { return nil }
func (r *handlerRepo) GetArtifact(_ context.Context, tenant, runID, artType string) ([]byte, string, error) {
	r.lastTenant = tenant
	r.lastArtifactType = artType
	return r.artifact, r.artCT, r.artErr
}
func (r *handlerRepo) CompareModels(_ context.Context, _, _, _, _ string) ([]ScenarioModelComparison, error) {
	return nil, nil
}
func (r *handlerRepo) ModelMatrix(_ context.Context, _ string, _, _ []string, evidenceMode string) (*bench.ModelMatrix, error) {
	r.lastMode = evidenceMode
	return r.matrix, r.matrixErr
}
func (r *handlerRepo) SignalSummary(_ context.Context, tenant string, f bench.RunFilters) (*bench.SignalAggregation, error) {
	r.lastTenant = tenant
	r.lastFilter = f
	return r.signals, r.signalsErr
}
func (r *handlerRepo) Regressions(_ context.Context, tenant string) ([]bench.Regression, error) {
	r.lastTenant = tenant
	return r.regressions, r.regressErr
}
func (r *handlerRepo) FailureAnalysis(_ context.Context, tenant string, _ string) (*bench.FailureInsights, error) {
	r.lastTenant = tenant
	return r.insights, r.insightsErr
}
func (r *handlerRepo) UpsertScenarios(_ context.Context, _ []bench.ScenarioSummary) (int, error) {
	return 0, nil
}
func (r *handlerRepo) RegisterRunner(_ context.Context, _ string, _ RegisterRunnerRequest) (*Runner, error) {
	return r.registeredRunner, nil
}
func (r *handlerRepo) ListRunners(context.Context, string) ([]Runner, error) { return r.runners, nil }
func (r *handlerRepo) DeleteRunner(context.Context, string, string) error    { return nil }
func (r *handlerRepo) TouchRunner(context.Context, string, string) error     { return nil }
func (r *handlerRepo) ClaimJob(_ context.Context, _ string, _ string, _ []string) (*BenchJob, error) {
	return r.claimedJob, nil
}
func (r *handlerRepo) CompleteJob(context.Context, string, string, string, string, int, int, string) error {
	return nil
}
func (r *handlerRepo) FindRunnerForModel(_ context.Context, _ string, _ string) (*Runner, error) {
	return r.foundRunner, nil
}
func (r *handlerRepo) MarkUnhealthyRunners(_ context.Context, _ time.Duration) (int, error) {
	return 0, nil
}
func (r *handlerRepo) ResetStaleJobs(_ context.Context, _ time.Duration) (int, error) {
	return 0, nil
}
func (r *handlerRepo) UpdateJobProgress(_ context.Context, _ string, _, _, _ int) error {
	return nil
}
func (r *handlerRepo) EnqueueJob(_ context.Context, _ string, _ string, _ string, cfg JobConfig) (*BenchJob, error) {
	r.lastEnqueueCfg = cfg
	if r.enqueuedJob != nil {
		return r.enqueuedJob, nil
	}
	return &BenchJob{ID: "job-enq-1", Status: "queued"}, nil
}
func (r *handlerRepo) BeginTx(_ context.Context) (pgx.Tx, error) {
	return nil, fmt.Errorf("handlerRepo: no real tx")
}

// passthroughAuth sets the given tenant on the request context without checking tokens.
func passthroughAuth(tenantID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithTenantID(r.Context(), tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// setupMux creates a mux with RegisterRoutes using the given repo and config.
func setupMux(repo *handlerRepo, cfg ServiceConfig, tenantID string) *http.ServeMux {
	svc := NewService(repo, cfg)
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth(tenantID))
	return mux
}

func rejectingAuth(http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "auth required", http.StatusUnauthorized)
	})
}

func TestRegisterRoutes_PublicReadEndpointsUsePublicTenantWithoutAuth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
	}{
		{name: "scenarios", path: "/v1/bench/scenarios"},
		{name: "runs", path: "/v1/bench/runs"},
		{name: "stats", path: "/v1/bench/stats"},
		{name: "catalog", path: "/v1/bench/catalog"},
		{name: "signals", path: "/v1/bench/signals"},
		{name: "regressions", path: "/v1/bench/regressions"},
		{name: "insights", path: "/v1/bench/insights?scenario=s1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &handlerRepo{
				runs:    []bench.RunRecord{{ID: "r1", ScenarioID: "s1", Model: "sonnet"}},
				catalog: &bench.RunCatalog{Models: []string{"sonnet"}, Providers: []string{"anthropic"}},
				signals: &bench.SignalAggregation{Signals: map[string]bench.SignalCount{}},
				insights: &bench.FailureInsights{
					ScenarioID: "s1",
				},
			}
			svc := NewService(repo, ServiceConfig{PublicTenant: "bench-public"})
			mux := http.NewServeMux()
			RegisterRoutes(mux, svc, rejectingAuth)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
			}
			if tt.name != "scenarios" && repo.lastTenant != "bench-public" {
				t.Fatalf("tenant = %q, want bench-public", repo.lastTenant)
			}
		})
	}
}

func TestRegisterRoutes_PublicReadEndpointsRequirePublicTenant(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	svc := NewService(repo, ServiceConfig{})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, rejectingAuth)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/bench/runs", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
}

func evidenceModeMatchesQuery(mode, stored string) bool {
	return mode == "" || stored == mode
}

func filterRunsByEvidenceMode(runs []bench.RunRecord, mode string) []bench.RunRecord {
	filtered := make([]bench.RunRecord, 0, len(runs))
	for _, run := range runs {
		if evidenceModeMatchesQuery(mode, run.EvidenceMode) {
			filtered = append(filtered, run)
		}
	}
	return filtered
}

func filterRunsByToolServer(runs []bench.RunRecord, toolServer string) []bench.RunRecord {
	if toolServer == "" {
		return runs
	}
	filtered := make([]bench.RunRecord, 0, len(runs))
	for _, run := range runs {
		if run.ToolServer == toolServer {
			filtered = append(filtered, run)
		}
	}
	return filtered
}

func filterRunsByScenarioIDs(runs []bench.RunRecord, scenarios []string) []bench.RunRecord {
	if len(scenarios) == 0 {
		return runs
	}
	allowed := make(map[string]struct{}, len(scenarios))
	for _, scenario := range scenarios {
		allowed[scenario] = struct{}{}
	}
	filtered := make([]bench.RunRecord, 0, len(runs))
	for _, run := range runs {
		if _, ok := allowed[run.ScenarioID]; ok {
			filtered = append(filtered, run)
		}
	}
	return filtered
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func aggregateLeaderboardRuns(runs []bench.RunRecord) []bench.LeaderboardEntry {
	type agg struct {
		scenarios map[string]struct{}
		runs      int
		passed    int
		duration  float64
		cost      float64
	}

	byModel := make(map[string]*agg)
	for _, run := range runs {
		entry := byModel[run.Model]
		if entry == nil {
			entry = &agg{scenarios: make(map[string]struct{})}
			byModel[run.Model] = entry
		}
		entry.scenarios[run.ScenarioID] = struct{}{}
		entry.runs++
		if run.Passed {
			entry.passed++
		}
		entry.duration += run.Duration
		entry.cost += run.EstimatedCost
	}

	out := make([]bench.LeaderboardEntry, 0, len(byModel))
	for model, entry := range byModel {
		runsCount := entry.runs
		passRate := 0.0
		if runsCount > 0 {
			passRate = 100.0 * float64(entry.passed) / float64(runsCount)
		}
		avgDuration := 0.0
		avgCost := 0.0
		if runsCount > 0 {
			avgDuration = entry.duration / float64(runsCount)
			avgCost = entry.cost / float64(runsCount)
		}
		out = append(out, bench.LeaderboardEntry{
			Model:       model,
			Scenarios:   len(entry.scenarios),
			Runs:        runsCount,
			PassRate:    passRate,
			AvgDuration: avgDuration,
			AvgCost:     avgCost,
			TotalCost:   entry.cost,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].PassRate != out[j].PassRate {
			return out[i].PassRate > out[j].PassRate
		}
		return out[i].Model < out[j].Model
	})
	return out
}

func aggregateStatsRuns(runs []bench.RunRecord) *bench.StatsResult {
	out := &bench.StatsResult{}
	for _, run := range runs {
		out.TotalRuns++
		if run.Passed {
			out.PassCount++
		} else {
			out.FailCount++
		}
	}
	return out
}
