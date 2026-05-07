package benchsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

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
	return aggregateStatsRuns(filterRunsByEvidenceMode(r.runs, f.EvidenceMode)), nil
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
func (r *handlerRepo) Leaderboard(_ context.Context, tenant, mode string, _ int) ([]bench.LeaderboardEntry, error) {
	r.lastTenant = tenant
	r.lastMode = mode
	if r.leadersErr != nil {
		return nil, r.leadersErr
	}
	if r.leaders != nil {
		return r.leaders, nil
	}
	return aggregateLeaderboardRuns(filterRunsByEvidenceMode(r.runs, mode)), nil
}
func (r *handlerRepo) ListScenarios(_ context.Context) ([]bench.ScenarioSummary, error) {
	return r.scenarios, r.scenErr
}
func (r *handlerRepo) StoreArtifact(_ context.Context, _, _, _ string, _ []byte) error { return nil }
func (r *handlerRepo) GetArtifact(_ context.Context, tenant, runID, artType string) ([]byte, string, error) {
	r.lastTenant = tenant
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
	switch mode {
	case "":
		return true
	case "evidra":
		return stored != "none"
	default:
		return stored == mode
	}
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

// ---------- Leaderboard ----------

func TestHandleLeaderboard_ReturnsModels(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		leaders: []bench.LeaderboardEntry{
			{Model: "sonnet", Runs: 10, PassRate: 0.9},
			{Model: "opus", Runs: 5, PassRate: 1.0},
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "t1")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/leaderboard", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if _, ok := body["models"]; !ok {
		t.Fatal("response missing 'models' key")
	}
	var models []bench.LeaderboardEntry
	if err := json.Unmarshal(body["models"], &models); err != nil {
		t.Fatalf("decode models: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("len(models) = %d, want 2", len(models))
	}
}

func TestHandleLeaderboard_DefaultsToProxy(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "t1")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/leaderboard", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if repo.lastMode != "" {
		t.Fatalf("evidence_mode = %q, want empty (all)", repo.lastMode)
	}

	var body struct {
		EvidenceMode string `json:"evidence_mode"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.EvidenceMode != "" {
		t.Fatalf("response evidence_mode = %q, want empty", body.EvidenceMode)
	}
}

func TestHandleLeaderboard_EvidenceModeFiltersAndAggregates(t *testing.T) {
	t.Parallel()

	sharedRuns := []bench.RunRecord{
		{ID: "baseline-1", ScenarioID: "s1", Model: "sonnet", EvidenceMode: "none", Passed: true, Duration: 10, EstimatedCost: 1.0},
		{ID: "baseline-2", ScenarioID: "s2", Model: "sonnet", EvidenceMode: "none", Passed: false, Duration: 20, EstimatedCost: 2.0},
		{ID: "evidra-1", ScenarioID: "s1", Model: "sonnet", EvidenceMode: "smart", Passed: true, Duration: 30, EstimatedCost: 3.0},
		{ID: "evidra-2", ScenarioID: "s2", Model: "sonnet", EvidenceMode: "direct", Passed: false, Duration: 40, EstimatedCost: 4.0},
	}

	tests := []struct {
		name         string
		mode         string
		wantRuns     int
		wantPassRate float64
	}{
		{name: "baseline only", mode: "none", wantRuns: 2, wantPassRate: 50.0},
		{name: "non-baseline alias", mode: "evidra", wantRuns: 2, wantPassRate: 50.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := &handlerRepo{runs: sharedRuns}
			mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "t1")

			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/bench/leaderboard?evidence_mode="+tt.mode, nil)
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}

			var body struct {
				Models       []bench.LeaderboardEntry `json:"models"`
				EvidenceMode string                   `json:"evidence_mode"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body.EvidenceMode != tt.mode {
				t.Fatalf("response evidence_mode = %q, want %q", body.EvidenceMode, tt.mode)
			}
			if len(body.Models) != 1 {
				t.Fatalf("len(models) = %d, want 1", len(body.Models))
			}
			if body.Models[0].Model != "sonnet" {
				t.Fatalf("model = %q, want sonnet", body.Models[0].Model)
			}
			if body.Models[0].Runs != tt.wantRuns {
				t.Fatalf("runs = %d, want %d", body.Models[0].Runs, tt.wantRuns)
			}
			if body.Models[0].PassRate != tt.wantPassRate {
				t.Fatalf("pass_rate = %v, want %v", body.Models[0].PassRate, tt.wantPassRate)
			}
		})
	}
}

func TestHandleLeaderboard_503WhenNoPublicTenant(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: ""}, "t1")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/leaderboard", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

// ---------- Models ----------

func TestHandleListModels_ReturnsModels(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		enabledModels: []EnabledModel{
			{
				ID:                "gemini-2.5-flash",
				DisplayName:       "Gemini 2.5 Flash",
				Provider:          "google",
				InputCostPerMtok:  0.15,
				OutputCostPerMtok: 0.60,
			},
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/models", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body struct {
		Models []EnabledModel `json:"models"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(body.Models))
	}
	if body.Models[0].ID != "gemini-2.5-flash" {
		t.Fatalf("id = %q, want gemini-2.5-flash", body.Models[0].ID)
	}
	if repo.lastTenant != "tenant-a" {
		t.Fatalf("tenant = %q, want tenant-a", repo.lastTenant)
	}
}

func TestHandleUpsertTenantProvider_Returns404WhileDisabled(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/v1/bench/models/gemini-2.5-flash/provider", strings.NewReader(`{"api_key":"sk-secret","rate_limit":10}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	if repo.lastTenant != "" {
		t.Fatalf("tenant = %q, want empty because route is disabled", repo.lastTenant)
	}
	if repo.lastModelID != "" {
		t.Fatalf("modelID = %q, want empty because route is disabled", repo.lastModelID)
	}
}

func TestHandleDeleteTenantProvider_Returns404WhileDisabled(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/v1/bench/models/gemini-2.5-flash/provider", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	if repo.lastTenant != "" {
		t.Fatalf("tenant = %q, want empty because route is disabled", repo.lastTenant)
	}
	if repo.lastModelID != "" {
		t.Fatalf("modelID = %q, want empty because route is disabled", repo.lastModelID)
	}
}

func TestHandleUpdateGlobalModel_Returns204(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	svc := NewService(repo, ServiceConfig{})
	handler := HandleUpdateGlobalModel(svc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/v1/admin/bench/models/gemini-2.5-flash", strings.NewReader(`{"api_key_env":"CUSTOM_KEY","api_base_url":"https://gw.example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("model_id", "gemini-2.5-flash")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if repo.lastModelID != "gemini-2.5-flash" {
		t.Fatalf("modelID = %q, want gemini-2.5-flash", repo.lastModelID)
	}
	if repo.lastGlobalCfg.APIKeyEnv != "CUSTOM_KEY" {
		t.Fatalf("api_key_env = %q, want CUSTOM_KEY", repo.lastGlobalCfg.APIKeyEnv)
	}
	if repo.lastGlobalCfg.APIBaseURL != "https://gw.example.com" {
		t.Fatalf("api_base_url = %q, want https://gw.example.com", repo.lastGlobalCfg.APIBaseURL)
	}
}

// ---------- List Runs ----------

func TestHandleListRuns_ReturnsItems(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		runs: []bench.RunRecord{
			{ID: "r1", ScenarioID: "s1", Model: "sonnet"},
			{ID: "r2", ScenarioID: "s2", Model: "opus"},
		},
		runsTotal: 2,
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body struct {
		Items  []bench.RunRecord `json:"runs"`
		Total  int               `json:"total"`
		Limit  int               `json:"limit"`
		Offset int               `json:"offset"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(body.Items))
	}
	if body.Total != 2 {
		t.Fatalf("total = %d, want 2", body.Total)
	}
	if body.Limit != 50 {
		t.Fatalf("limit = %d, want 50 (default)", body.Limit)
	}
	if repo.lastTenant != "pub" {
		t.Fatalf("tenant = %q, want pub", repo.lastTenant)
	}
}

func TestHandleListRuns_ParsesFilters(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{runsTotal: 0}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-b")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs?model=sonnet&scenario=broken-deployment&evidence_mode=direct&limit=10&offset=5", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	f := repo.lastFilter
	if f.Model != "sonnet" {
		t.Errorf("Model = %q, want sonnet", f.Model)
	}
	if f.ScenarioID != "broken-deployment" {
		t.Errorf("ScenarioID = %q, want broken-deployment", f.ScenarioID)
	}
	if f.EvidenceMode != "direct" {
		t.Errorf("EvidenceMode = %q, want direct", f.EvidenceMode)
	}
	if f.Limit != 10 {
		t.Errorf("Limit = %d, want 10", f.Limit)
	}
	if f.Offset != 5 {
		t.Errorf("Offset = %d, want 5", f.Offset)
	}
}

func TestHandleListRuns_EvidenceModeFiltersItems(t *testing.T) {
	t.Parallel()

	sharedRuns := []bench.RunRecord{
		{ID: "baseline-1", ScenarioID: "s1", Model: "sonnet", EvidenceMode: "none"},
		{ID: "baseline-2", ScenarioID: "s2", Model: "sonnet", EvidenceMode: "none"},
		{ID: "evidra-1", ScenarioID: "s3", Model: "sonnet", EvidenceMode: "smart"},
		{ID: "evidra-2", ScenarioID: "s4", Model: "sonnet", EvidenceMode: "direct"},
	}

	tests := []struct {
		name    string
		mode    string
		wantIDs []string
	}{
		{name: "baseline only", mode: "none", wantIDs: []string{"baseline-1", "baseline-2"}},
		{name: "non-baseline alias", mode: "evidra", wantIDs: []string{"evidra-1", "evidra-2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := &handlerRepo{runs: sharedRuns}
			mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/bench/runs?evidence_mode="+tt.mode, nil)
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			var body struct {
				Items []bench.RunRecord `json:"runs"`
				Total int               `json:"total"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body.Total != len(tt.wantIDs) {
				t.Fatalf("total = %d, want %d", body.Total, len(tt.wantIDs))
			}
			if len(body.Items) != len(tt.wantIDs) {
				t.Fatalf("len(items) = %d, want %d", len(body.Items), len(tt.wantIDs))
			}
			for i, wantID := range tt.wantIDs {
				if body.Items[i].ID != wantID {
					t.Fatalf("items[%d].ID = %q, want %q", i, body.Items[i].ID, wantID)
				}
			}
		})
	}
}

// ---------- Get Run ----------

func TestHandleGetRun_ReturnsRecord(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		run: &bench.RunRecord{ID: "run-42", ScenarioID: "s1", Model: "sonnet", Passed: true},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs/run-42", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var run bench.RunRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &run); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if run.ID != "run-42" {
		t.Fatalf("ID = %q, want run-42", run.ID)
	}
}

func TestHandleGetRun_404ForMissing(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{runErr: ErrNotFound}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs/nonexistent", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// ---------- Ingest ----------

func TestHandleIngestRun_ValidPayload(t *testing.T) {
	t.Parallel()

	// IngestRun calls BeginTx which our handlerRepo doesn't support,
	// so we use a dedicated repo that returns a fakeTx.
	repo := &ingestRepo{}
	svc := NewService(repo, ServiceConfig{PublicTenant: "pub"})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("tenant-a"))

	payload := `{"id":"r1","scenario_id":"s1","model":"sonnet"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/bench/runs", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["ok"] != true {
		t.Fatalf("ok = %v, want true", body["ok"])
	}
}

func TestHandleIngestRun_RejectsMissingFields(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	tests := []struct {
		name    string
		payload string
	}{
		{"missing id", `{"scenario_id":"s1","model":"m1"}`},
		{"missing scenario_id", `{"id":"r1","model":"m1"}`},
		{"missing model", `{"id":"r1","scenario_id":"s1"}`},
		{"empty body", `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/bench/runs", strings.NewReader(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
		})
	}
}

func TestHandleIngestBatch_ImportsRuns(t *testing.T) {
	t.Parallel()

	repo := &ingestRepo{batchCount: 3}
	svc := NewService(repo, ServiceConfig{PublicTenant: "pub"})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("tenant-a"))

	payload := `{"runs":[
		{"id":"r1","scenario_id":"s1","model":"m1"},
		{"id":"r2","scenario_id":"s1","model":"m1"},
		{"id":"r3","scenario_id":"s2","model":"m2"}
	]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/bench/runs/batch", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["ok"] != true {
		t.Fatalf("ok = %v, want true", body["ok"])
	}
	if int(body["imported"].(float64)) != 3 {
		t.Fatalf("imported = %v, want 3", body["imported"])
	}
}

// ---------- Artifacts ----------

func TestHandleGetTranscript_ReturnsText(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		artifact: []byte("step 1\nstep 2\nstep 3"),
		artCT:    "text/plain",
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs/r1/transcript", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain" {
		t.Fatalf("Content-Type = %q, want text/plain", ct)
	}
	if rec.Body.String() != "step 1\nstep 2\nstep 3" {
		t.Fatalf("body = %q, want transcript text", rec.Body.String())
	}
}

func TestHandleGetTranscript_404WhenMissing(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{artErr: ErrNotFound}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs/r1/transcript", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleGetTimeline_ComputesPhases(t *testing.T) {
	t.Parallel()

	toolCalls := []bench.ToolCall{
		{Tool: "run_command", Args: json.RawMessage(`{"command":"kubectl get pods -n default"}`)},
		{Tool: "run_command", Args: json.RawMessage(`{"command":"kubectl describe pod/web -n default"}`)},
		{Tool: "run_command", Args: json.RawMessage(`{"command":"kubectl apply -f fix.yaml -n default"}`)},
		{Tool: "run_command", Args: json.RawMessage(`{"command":"kubectl get pods -n default"}`)},
	}
	data, _ := json.Marshal(toolCalls)

	repo := &handlerRepo{
		artifact: data,
		artCT:    "application/json",
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs/r1/timeline", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var tl bench.Timeline
	if err := json.Unmarshal(rec.Body.Bytes(), &tl); err != nil {
		t.Fatalf("decode timeline: %v", err)
	}
	if tl.TotalSteps != 4 {
		t.Fatalf("TotalSteps = %d, want 4", tl.TotalSteps)
	}
	if tl.MutationCount != 1 {
		t.Fatalf("MutationCount = %d, want 1", tl.MutationCount)
	}
	// First call is discover, second is diagnose, third is act, fourth is verify.
	wantPhases := []bench.Phase{bench.PhaseDiscover, bench.PhaseDiagnose, bench.PhaseAct, bench.PhaseVerify}
	for i, want := range wantPhases {
		if tl.Steps[i].Phase != want {
			t.Errorf("step[%d].Phase = %q, want %q", i, tl.Steps[i].Phase, want)
		}
	}
}

func TestHandleGetTimeline_404WhenNoToolCalls(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{artErr: ErrNotFound}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs/r1/timeline", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// ---------- Stats / Catalog / Scenarios ----------

func TestHandleStats_ReturnsAggregates(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		stats: &bench.StatsResult{
			TotalRuns: 42,
			PassCount: 38,
			FailCount: 4,
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/stats", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body bench.StatsResult
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.TotalRuns != 42 {
		t.Fatalf("TotalRuns = %d, want 42", body.TotalRuns)
	}
}

func TestHandleStats_EvidenceModeFiltersTotals(t *testing.T) {
	t.Parallel()

	sharedRuns := []bench.RunRecord{
		{ID: "baseline-1", ScenarioID: "s1", Model: "sonnet", EvidenceMode: "none", Passed: true},
		{ID: "baseline-2", ScenarioID: "s2", Model: "sonnet", EvidenceMode: "none", Passed: false},
		{ID: "evidra-1", ScenarioID: "s3", Model: "sonnet", EvidenceMode: "smart", Passed: true},
		{ID: "evidra-2", ScenarioID: "s4", Model: "sonnet", EvidenceMode: "direct", Passed: false},
	}

	tests := []struct {
		name      string
		mode      string
		wantTotal int
		wantPass  int
		wantFail  int
	}{
		{name: "baseline only", mode: "none", wantTotal: 2, wantPass: 1, wantFail: 1},
		{name: "non-baseline alias", mode: "evidra", wantTotal: 2, wantPass: 1, wantFail: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := &handlerRepo{runs: sharedRuns}
			mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/bench/stats?evidence_mode="+tt.mode, nil)
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			var body bench.StatsResult
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body.TotalRuns != tt.wantTotal {
				t.Fatalf("TotalRuns = %d, want %d", body.TotalRuns, tt.wantTotal)
			}
			if body.PassCount != tt.wantPass {
				t.Fatalf("PassCount = %d, want %d", body.PassCount, tt.wantPass)
			}
			if body.FailCount != tt.wantFail {
				t.Fatalf("FailCount = %d, want %d", body.FailCount, tt.wantFail)
			}
		})
	}
}

func TestHandleCatalog_ReturnsModelsAndProviders(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		catalog: &bench.RunCatalog{
			Models:    []string{"sonnet", "opus"},
			Providers: []string{"anthropic", "bifrost"},
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/catalog", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body bench.RunCatalog
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Models) != 2 {
		t.Fatalf("len(Models) = %d, want 2", len(body.Models))
	}
	if len(body.Providers) != 2 {
		t.Fatalf("len(Providers) = %d, want 2", len(body.Providers))
	}
}

func TestHandleListScenarios_ReturnsArray(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		scenarios: []bench.ScenarioSummary{
			{ID: "broken-deployment", Title: "Broken Deployment", Category: "kubectl"},
			{ID: "helm-rollback", Title: "Helm Rollback", Category: "helm"},
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/scenarios", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body struct {
		Scenarios []bench.ScenarioSummary `json:"scenarios"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Scenarios) != 2 {
		t.Fatalf("len(scenarios) = %d, want 2", len(body.Scenarios))
	}
}

// ---------- Ingest support: fake that supports transactions ----------

// ingestRepo wraps handlerRepo with a fake transaction that accepts Exec and Commit.
type ingestRepo struct {
	handlerRepo
	batchCount int
}

func (r *ingestRepo) UpsertScenarios(_ context.Context, _ []bench.ScenarioSummary) (int, error) {
	return 0, nil
}
func (r *ingestRepo) BeginTx(_ context.Context) (pgx.Tx, error) {
	// Reuse fakeTx from service_batch_test.go (same package).
	// Supply enough "INSERT 0 1" tags so IngestRunBatch counts rows as inserted.
	tags := make([]pgconn.CommandTag, 20)
	for i := range tags {
		tags[i] = pgconn.NewCommandTag("INSERT 0 1")
	}
	return &fakeTx{execTags: tags}, nil
}

func (r *ingestRepo) InsertRunBatch(_ context.Context, _ string, _ []bench.RunRecord) (int, error) {
	return r.batchCount, nil
}

// ---------- Delete ----------

func TestHandleDeleteRun_Returns204(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/v1/bench/runs/run-42", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if repo.lastTenant != "tenant-a" {
		t.Fatalf("tenant = %q, want tenant-a", repo.lastTenant)
	}
}

func TestHandleDeleteRun_404ForMissing(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{deleteErr: ErrNotFound}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/v1/bench/runs/nonexistent", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// ---------- Archive ----------

func TestHandleArchiveRuns_ReturnsCount(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{archiveCount: 5}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	payload := `{"model":"sonnet"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/bench/runs/archive", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if int(body["archived"].(float64)) != 5 {
		t.Fatalf("archived = %v, want 5", body["archived"])
	}
}

func TestHandleArchiveRuns_RejectsEmptyFilter(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	payload := `{}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/bench/runs/archive", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandleArchiveRuns_AcceptsBeforeFilter(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{archiveCount: 10}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	payload := `{"before":"2026-03-21T00:00:00Z"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/bench/runs/archive", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestHandleArchiveRuns_AcceptsIDsFilter(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{archiveCount: 2}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	payload := `{"ids":["run-1","run-2"]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/bench/runs/archive", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

// ---------- Compare ----------

func TestHandleCompareRuns_ReturnsDelta(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		run: &bench.RunRecord{
			ID:               "run-1",
			ScenarioID:       "s1",
			Model:            "sonnet",
			Passed:           true,
			Duration:         30.0,
			Turns:            5,
			EstimatedCost:    0.10,
			PromptTokens:     1000,
			CompletionTokens: 500,
			ChecksPassed:     3,
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/compare/runs?a=run-1&b=run-1", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var cmp RunComparison
	if err := json.Unmarshal(rec.Body.Bytes(), &cmp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cmp.RunA.ID != "run-1" {
		t.Fatalf("RunA.ID = %q, want run-1", cmp.RunA.ID)
	}
	if cmp.Delta.PassedChanged {
		t.Fatal("PassedChanged = true, want false (same run)")
	}
	if cmp.Delta.DurationDiff != 0 {
		t.Fatalf("DurationDiff = %f, want 0", cmp.Delta.DurationDiff)
	}
}

func TestHandleCompareModels_ReturnsComparison(t *testing.T) {
	t.Parallel()

	repo := &compareModelsRepo{
		scenarios: []ScenarioModelComparison{
			{ScenarioID: "broken-deployment", APassRate: 100, BPassRate: 80, ACost: 0.10, BCost: 0.20},
		},
	}
	svc := NewService(repo, ServiceConfig{PublicTenant: "pub"})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("tenant-a"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/compare/models?a=sonnet&b=opus", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var cmp ModelComparison
	if err := json.Unmarshal(rec.Body.Bytes(), &cmp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cmp.ModelA != "sonnet" {
		t.Fatalf("ModelA = %q, want sonnet", cmp.ModelA)
	}
	if cmp.ModelB != "opus" {
		t.Fatalf("ModelB = %q, want opus", cmp.ModelB)
	}
	if len(cmp.Scenarios) != 1 {
		t.Fatalf("len(Scenarios) = %d, want 1", len(cmp.Scenarios))
	}
	if cmp.Summary.SharedScenarios != 1 {
		t.Fatalf("SharedScenarios = %d, want 1", cmp.Summary.SharedScenarios)
	}
	if repo.lastMode != "" {
		t.Fatalf("evidence_mode = %q, want empty", repo.lastMode)
	}
}

func TestHandleCompareModels_MatrixPassesEvidenceMode(t *testing.T) {
	t.Parallel()

	repo := &matrixRepo{
		matrix: &bench.ModelMatrix{
			Models:    []string{"sonnet"},
			Scenarios: []string{"broken-deployment"},
		},
	}
	svc := NewService(repo, ServiceConfig{PublicTenant: "pub"})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("tenant-a"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/compare/models?models=sonnet&scenarios=broken-deployment&evidence_mode=evidra", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if repo.lastMode != "evidra" {
		t.Fatalf("evidence_mode = %q, want evidra", repo.lastMode)
	}
}

// compareModelsRepo is a fake that returns canned CompareModels data.
type compareModelsRepo struct {
	handlerRepo
	scenarios []ScenarioModelComparison
}

func (r *compareModelsRepo) CompareModels(_ context.Context, _, _, _, evidenceMode string) ([]ScenarioModelComparison, error) {
	r.lastMode = evidenceMode
	return r.scenarios, nil
}
func (r *compareModelsRepo) ModelMatrix(_ context.Context, _ string, _, _ []string, evidenceMode string) (*bench.ModelMatrix, error) {
	r.lastMode = evidenceMode
	return nil, nil
}
func (r *compareModelsRepo) SignalSummary(_ context.Context, _ string, _ bench.RunFilters) (*bench.SignalAggregation, error) {
	return nil, nil
}
func (r *compareModelsRepo) Regressions(_ context.Context, _ string) ([]bench.Regression, error) {
	return nil, nil
}
func (r *compareModelsRepo) FailureAnalysis(_ context.Context, _ string, _ string) (*bench.FailureInsights, error) {
	return nil, nil
}
func (r *compareModelsRepo) UpsertScenarios(_ context.Context, _ []bench.ScenarioSummary) (int, error) {
	return 0, nil
}

// ---------- Signals ----------

func TestHandleSignals_ReturnsAggregation(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		signals: &bench.SignalAggregation{
			TotalRuns:         10,
			RunsWithScorecard: 8,
			AvgScore:          75.5,
			Signals: map[string]bench.SignalCount{
				"artifact_drift": {Total: 5, RunCount: 3},
			},
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/signals?model=sonnet", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body bench.SignalAggregation
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.TotalRuns != 10 {
		t.Fatalf("TotalRuns = %d, want 10", body.TotalRuns)
	}
	if body.RunsWithScorecard != 8 {
		t.Fatalf("RunsWithScorecard = %d, want 8", body.RunsWithScorecard)
	}
	if repo.lastFilter.Model != "sonnet" {
		t.Fatalf("filter.Model = %q, want sonnet", repo.lastFilter.Model)
	}
}

func TestHandleSignals_SinceFilterParsed(t *testing.T) {
	t.Parallel()

	// Go 1.20+ parses fractional-second RFC3339 timestamps with time.RFC3339.
	// Verify parseSince propagates a non-nil Since filter so the query includes
	// bench_runs.created_at >= $N (not an unqualified created_at that would be
	// ambiguous when joined with bench_artifacts).
	repo := &handlerRepo{signals: &bench.SignalAggregation{TotalRuns: 5}}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/signals?since=2026-04-06T09:56:11.778Z", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if repo.lastFilter.Since == nil {
		t.Fatal("Since filter not propagated: parseSince returned nil for fractional-second timestamp")
	}
	want := "2026-04-06 09:56:11.778 +0000 UTC"
	if got := repo.lastFilter.Since.String(); got != want {
		t.Fatalf("Since = %q, want %q", got, want)
	}
}

// ---------- Regressions ----------

func TestHandleRegressions_ReturnsArray(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		regressions: []bench.Regression{
			{ScenarioID: "broken-deployment", Model: "sonnet", LatestRunID: "r1", Severity: "critical", PrevRate: 90},
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/regressions", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body []bench.Regression
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("len(regressions) = %d, want 1", len(body))
	}
	if body[0].Severity != "critical" {
		t.Fatalf("severity = %q, want critical", body[0].Severity)
	}
}

func TestHandleRegressions_EmptyReturnsEmptyArray(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/regressions", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "[]" && rec.Body.String() != "[]\n" {
		t.Fatalf("body = %q, want empty array", rec.Body.String())
	}
}

// ---------- Failure Analysis ----------

func TestHandleFailureAnalysis_ReturnsInsights(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		insights: &bench.FailureInsights{
			ScenarioID: "broken-deployment",
			TotalRuns:  20,
			FailedRuns: 8,
			PassedRuns: 12,
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/insights?scenario=broken-deployment", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body bench.FailureInsights
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ScenarioID != "broken-deployment" {
		t.Fatalf("ScenarioID = %q, want broken-deployment", body.ScenarioID)
	}
	if body.TotalRuns != 20 {
		t.Fatalf("TotalRuns = %d, want 20", body.TotalRuns)
	}
}

func TestHandleFailureAnalysis_RequiresScenario(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/insights", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// ---------- Compare Models (multi-model) ----------

func TestHandleCompareModels_AcceptsModelsParam(t *testing.T) {
	t.Parallel()

	matrixRepo := &matrixRepo{
		matrix: &bench.ModelMatrix{
			Models:    []string{"opus", "sonnet"},
			Scenarios: []string{"broken-deployment"},
			Cells: map[string]map[string]bench.ModelMatrixCell{
				"broken-deployment": {
					"sonnet": {Runs: 5, Passed: 4, PassRate: 80},
					"opus":   {Runs: 3, Passed: 3, PassRate: 100},
				},
			},
		},
	}
	svc := NewService(matrixRepo, ServiceConfig{PublicTenant: "pub"})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("tenant-a"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/compare/models?models=sonnet,opus&scenarios=broken-deployment", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body bench.ModelMatrix
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Models) != 2 {
		t.Fatalf("len(Models) = %d, want 2", len(body.Models))
	}
	if len(body.Scenarios) != 1 {
		t.Fatalf("len(Scenarios) = %d, want 1", len(body.Scenarios))
	}
}

func TestHandleCompareModels_RejectsNoParams(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/compare/models", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// matrixRepo is a fake that returns canned ModelMatrix data.
type matrixRepo struct {
	handlerRepo
	matrix   *bench.ModelMatrix
	lastMode string
}

func (r *matrixRepo) ModelMatrix(_ context.Context, _ string, _, _ []string, evidenceMode string) (*bench.ModelMatrix, error) {
	r.lastMode = evidenceMode
	return r.matrix, nil
}
func (r *matrixRepo) CompareModels(_ context.Context, _, _, _, _ string) ([]ScenarioModelComparison, error) {
	return nil, nil
}
func (r *matrixRepo) SignalSummary(_ context.Context, _ string, _ bench.RunFilters) (*bench.SignalAggregation, error) {
	return nil, nil
}
func (r *matrixRepo) Regressions(_ context.Context, _ string) ([]bench.Regression, error) {
	return nil, nil
}
func (r *matrixRepo) FailureAnalysis(_ context.Context, _ string, _ string) (*bench.FailureInsights, error) {
	return nil, nil
}
func (r *matrixRepo) UpsertScenarios(_ context.Context, _ []bench.ScenarioSummary) (int, error) {
	return 0, nil
}

// ---------- Trigger ----------

// spyExecutor records whether Start was called.
type spyExecutor struct {
	started   bool
	job       *TriggerJob
	startedCh chan struct{}
}

func (e *spyExecutor) Start(_ context.Context, job *TriggerJob, _, _ string) error {
	e.started = true
	e.job = job
	if e.startedCh != nil {
		close(e.startedCh)
		e.startedCh = nil
	}
	return nil
}

func TestHandleTrigger_NoExecutor_Returns501(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "bifrost"},
	}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Executor:     nil, // no executor
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"test-model","evidence_mode":"smart","scenarios":["s1"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNotImplemented, rec.Body.String())
	}
}

func TestHandleTrigger_RequiresEvidenceMode(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "bifrost"},
	}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Executor:     &spyExecutor{},
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"test-model","scenarios":["s1","s2"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandleTrigger_RejectsInvalidEvidenceMode(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "bifrost"},
	}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Executor:     &spyExecutor{},
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"test-model","evidence_mode":"proxy","scenarios":["s1"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandleTrigger_RejectsInvalidExecutionMode(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "bifrost"},
	}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Executor:     &spyExecutor{},
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"test-model","execution_mode":"wat","evidence_mode":"smart","scenarios":["s1"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandleTrigger_ValidRequest_Returns202(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	startedCh := make(chan struct{})
	spy := &spyExecutor{startedCh: startedCh}
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "bifrost"},
	}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Executor:     spy,
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"test-model","evidence_mode":"smart","scenarios":["s1","s2"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := resp["id"]; !ok {
		t.Fatal("response missing 'id' key")
	}
	select {
	case <-startedCh:
	case <-time.After(time.Second):
		t.Fatal("executor Start was not called")
	}
	stored := store.Get(resp["id"].(string))
	if stored == nil {
		t.Fatal("stored trigger job missing")
	}
	if stored.EvidenceMode != "smart" {
		t.Fatalf("stored evidence mode = %q, want smart", stored.EvidenceMode)
	}
	if stored.ExecutionMode != "provider" {
		t.Fatalf("stored execution mode = %q, want provider", stored.ExecutionMode)
	}
	if spy.job == nil {
		t.Fatal("executor job missing")
	}
	if spy.job.EvidenceMode != "smart" {
		t.Fatalf("job evidence mode = %q, want smart", spy.job.EvidenceMode)
	}
	if spy.job.ExecutionMode != "provider" {
		t.Fatalf("job execution mode = %q, want provider", spy.job.ExecutionMode)
	}
}

func TestHandleTrigger_ValidRequest_Returns202_WithEvidenceModeNone(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "bifrost"},
	}
	spy := &spyExecutor{}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Executor:     spy,
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"sonnet","evidence_mode":"none","scenarios":["s1","s2"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	stored := store.Get(resp["id"].(string))
	if stored == nil {
		t.Fatal("stored trigger job missing")
	}
	if stored.EvidenceMode != "none" {
		t.Fatalf("stored evidence mode = %q, want none", stored.EvidenceMode)
	}
	if stored.ExecutionMode != "provider" {
		t.Fatalf("stored execution mode = %q, want provider", stored.ExecutionMode)
	}
}

func TestHandleTrigger_ValidRequest_Returns202_WithExecutionModeA2A(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	startedCh := make(chan struct{})
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "bifrost"},
	}
	spy := &spyExecutor{startedCh: startedCh}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Executor:     spy,
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"sonnet","execution_mode":"a2a","evidence_mode":"smart","scenarios":["s1","s2"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	stored := store.Get(resp["id"].(string))
	if stored == nil {
		t.Fatal("stored trigger job missing")
	}
	if stored.ExecutionMode != "a2a" {
		t.Fatalf("stored execution mode = %q, want a2a", stored.ExecutionMode)
	}
	select {
	case <-startedCh:
	case <-time.After(time.Second):
		t.Fatal("executor Start was not called")
	}
	if spy.job == nil {
		t.Fatal("executor job missing")
	}
	if spy.job.ExecutionMode != "a2a" {
		t.Fatalf("job execution mode = %q, want a2a", spy.job.ExecutionMode)
	}
}

func TestHandleTriggerProgress_UnknownJob_Returns404(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"scenario":"s1","status":"passed","completed":1,"total":1}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger/nonexistent/progress", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestHandleTriggerProgress_InvalidVersion_Returns400(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	// Create a job first so the 404 check passes.
	job := &TriggerJob{
		ID:     "job-1",
		Status: "running",
		Total:  1,
		Progress: []ScenarioProgress{
			{Scenario: "s1", Status: "running"},
		},
	}
	store.Create(job)

	rec := httptest.NewRecorder()
	body := `{"contract_version":"v2.0.0","scenario":"s1","status":"passed","completed":1,"total":1}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger/job-1/progress", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

// ---------- Runner Registration ----------

func TestHandleRegisterRunner_ValidModels(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		registeredRunner: &Runner{
			ID:     "runner-1",
			Status: "healthy",
			Config: RunnerConfig{Models: []string{"sonnet"}, PollInterval: 5},
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "t1")

	rec := httptest.NewRecorder()
	reqBody := `{"name":"my-runner","models":["sonnet"]}`
	req := httptest.NewRequest("POST", "/v1/runners/register", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["runner_id"] != "runner-1" {
		t.Fatalf("runner_id = %v, want runner-1", resp["runner_id"])
	}
	if resp["poll_interval"] != float64(5) {
		t.Fatalf("poll_interval = %v, want 5", resp["poll_interval"])
	}
}

func TestHandleRegisterRunner_MissingModels(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "t1")

	rec := httptest.NewRecorder()
	reqBody := `{"name":"my-runner"}`
	req := httptest.NewRequest("POST", "/v1/runners/register", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

// ---------- Trigger with Runner ----------

func TestHandleTrigger_WithRunner_QueuesJob(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "bifrost"},
		foundRunner: &Runner{
			ID:     "runner-1",
			Status: "healthy",
			Config: RunnerConfig{Models: []string{"sonnet"}},
		},
		enqueuedJob: &BenchJob{
			ID:       "job-q-1",
			Status:   "queued",
			Model:    "sonnet",
			Provider: "bifrost",
		},
		claimedJob: &BenchJob{
			ID:         "job-q-1",
			TenantID:   "pub",
			Model:      "sonnet",
			Provider:   "bifrost",
			Status:     "queued",
			ConfigJSON: json.RawMessage(`{"scenarios":["s1"],"evidence_mode":"smart","execution_mode":"a2a"}`),
		},
	}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Dispatcher:   &PoolDispatcher{},
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"sonnet","execution_mode":"a2a","evidence_mode":"smart","scenarios":["s1"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["id"] != "job-q-1" {
		t.Fatalf("id = %v, want job-q-1", resp["id"])
	}
	if resp["mode"] != "runner" {
		t.Fatalf("mode = %v, want runner", resp["mode"])
	}
	if repo.lastEnqueueCfg.EvidenceMode != "smart" {
		t.Fatalf("enqueue evidence mode = %q, want smart", repo.lastEnqueueCfg.EvidenceMode)
	}
	if repo.lastEnqueueCfg.ExecutionMode != "a2a" {
		t.Fatalf("enqueue execution mode = %q, want a2a", repo.lastEnqueueCfg.ExecutionMode)
	}
	stored := store.Get("job-q-1")
	if stored == nil {
		t.Fatal("stored runner trigger job missing")
	}
	if stored.EvidenceMode != "smart" {
		t.Fatalf("stored evidence mode = %q, want smart", stored.EvidenceMode)
	}
	if stored.ExecutionMode != "a2a" {
		t.Fatalf("stored execution mode = %q, want a2a", stored.ExecutionMode)
	}
}

func TestHandleTrigger_WithRunner_AllowsProviderSuppliedModelAlias(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{
		foundRunner: &Runner{
			ID:     "runner-1",
			Status: "healthy",
			Config: RunnerConfig{Models: []string{"sonnet"}, Provider: "claude"},
		},
		enqueuedJob: &BenchJob{
			ID:       "job-alias-1",
			Status:   "queued",
			Model:    "sonnet",
			Provider: "claude",
		},
	}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Dispatcher:   &PoolDispatcher{},
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"sonnet","provider":"claude","evidence_mode":"smart","scenarios":["s1"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if repo.lastEnqueueCfg.EvidenceMode != "smart" {
		t.Fatalf("enqueue evidence mode = %q, want smart", repo.lastEnqueueCfg.EvidenceMode)
	}
	stored := store.Get("job-alias-1")
	if stored == nil {
		t.Fatal("stored runner trigger job missing")
	}
	if stored.Provider != "claude" {
		t.Fatalf("stored provider = %q, want claude", stored.Provider)
	}
}

func TestHandleTrigger_WithRunner_DoesNotRequireControlPlaneModelAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")

	store := NewTriggerStore()
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "anthropic", APIKeyEnv: "ANTHROPIC_API_KEY"},
		foundRunner: &Runner{
			ID:     "runner-1",
			Status: "healthy",
			Config: RunnerConfig{Models: []string{"claude-sonnet-4-20250514"}, Provider: "anthropic"},
		},
		enqueuedJob: &BenchJob{
			ID:       "job-runner-key-1",
			Status:   "queued",
			Model:    "claude-sonnet-4-20250514",
			Provider: "anthropic",
		},
	}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Dispatcher:   &PoolDispatcher{},
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"claude-sonnet-4-20250514","evidence_mode":"smart","scenarios":["s1"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if stored := store.Get("job-runner-key-1"); stored == nil {
		t.Fatal("stored runner trigger job missing")
	}
}

func TestHandleTrigger_WithPinnedRunnerUnavailable_Returns400AndSkipsExecutor(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	spy := &spyExecutor{}
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "bifrost"},
		// No healthy runner for this model. This used to fall through to V1.
		runners: []Runner{
			{
				ID:     "runner-other",
				Status: "healthy",
				Config: RunnerConfig{Models: []string{"other-model"}},
			},
		},
	}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Executor:     spy,
		Dispatcher:   &PoolDispatcher{},
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"sonnet","runner_id":"runner-missing","evidence_mode":"smart","scenarios":["s1"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if spy.started {
		t.Fatal("executor should not start when pinned runner is unavailable")
	}
}

func TestHandleCompleteJob_UpdatesTriggerStoreSnapshot(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	store.Create(&TriggerJob{
		ID:       "job-complete-1",
		Status:   "pending",
		Model:    "sonnet",
		Provider: "claude",
		Total:    2,
		Progress: []ScenarioProgress{
			{Scenario: "s1", Status: "pending"},
			{Scenario: "s2", Status: "pending"},
		},
		CreatedAt: time.Now(),
	})

	svc := NewService(&handlerRepo{}, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"runner_id":"runner-1","status":"completed","passed":2,"failed":0}`
	req := httptest.NewRequest("POST", "/v1/runners/jobs/job-complete-1/complete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	got := store.Get("job-complete-1")
	if got == nil {
		t.Fatal("expected trigger job snapshot")
	}
	if got.Status != "completed" {
		t.Fatalf("trigger status = %q, want completed", got.Status)
	}
	if got.Completed != 2 || got.Passed != 2 || got.Failed != 0 {
		t.Fatalf("trigger counters = completed:%d passed:%d failed:%d, want 2/2/0", got.Completed, got.Passed, got.Failed)
	}
}

func TestHandlePollJob_ReturnsEvidenceMode(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		runners: []Runner{
			{
				ID:     "runner-1",
				Status: "healthy",
				Config: RunnerConfig{Models: []string{"sonnet"}},
			},
		},
		claimedJob: &BenchJob{
			ID:       "job-q-2",
			TenantID: "pub",
			Model:    "sonnet",
			Provider: "bifrost",
			Status:   "queued",
			ConfigJSON: json.RawMessage(`{
				"scenarios":["s1"],
				"runner_id":"runner-1",
				"evidence_mode":"smart",
				"execution_mode":"a2a"
			}`),
		},
	}
	svc := NewService(repo, ServiceConfig{PublicTenant: "pub"})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/runners/jobs?runner_id=runner-1", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["provider"] != "bifrost" {
		t.Fatalf("provider = %v, want bifrost", resp["provider"])
	}
	if resp["evidence_mode"] != "smart" {
		t.Fatalf("evidence_mode = %v, want smart", resp["evidence_mode"])
	}
	if resp["execution_mode"] != "a2a" {
		t.Fatalf("execution_mode = %v, want a2a", resp["execution_mode"])
	}
}

func TestHandlePollJob_DefaultsEvidenceModeForLegacyJobs(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		runners: []Runner{
			{ID: "runner-1", Status: "healthy", Config: RunnerConfig{Models: []string{"sonnet"}}},
		},
		claimedJob: &BenchJob{
			ID: "job-legacy", TenantID: "pub", Model: "sonnet", Provider: "bifrost",
			Status: "queued", ConfigJSON: json.RawMessage(`{"scenarios":["s1"]}`),
		},
	}
	svc := NewService(repo, ServiceConfig{PublicTenant: "pub"})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/runners/jobs?runner_id=runner-1", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["evidence_mode"] != "none" {
		t.Fatalf("evidence_mode = %v, want none", resp["evidence_mode"])
	}
	if resp["execution_mode"] != "provider" {
		t.Fatalf("execution_mode = %v, want provider", resp["execution_mode"])
	}
}

func TestHandlePollJob_RejectsMalformedConfigJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		configJSON string
	}{
		{
			name:       "malformed config json",
			configJSON: `{"scenarios":["s1"],"evidence_mode":`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := &handlerRepo{
				runners: []Runner{
					{
						ID:     "runner-1",
						Status: "healthy",
						Config: RunnerConfig{Models: []string{"sonnet"}},
					},
				},
				claimedJob: &BenchJob{
					ID:         "job-q-2",
					TenantID:   "pub",
					Model:      "sonnet",
					Provider:   "bifrost",
					Status:     "queued",
					ConfigJSON: json.RawMessage(tt.configJSON),
				},
			}
			svc := NewService(repo, ServiceConfig{PublicTenant: "pub"})
			mux := http.NewServeMux()
			RegisterRoutes(mux, svc, passthroughAuth("t1"))

			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/runners/jobs?runner_id=runner-1", nil)
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
		})
	}
}
