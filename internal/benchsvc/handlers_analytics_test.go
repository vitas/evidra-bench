package benchsvc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	bench "samebits.com/evidra-infra-bench/pkg/bench"
)

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
	req := httptest.NewRequest("GET", "/v1/bench/compare/models?models=sonnet&scenarios=broken-deployment&evidence_mode=mcp", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if repo.lastMode != "mcp" {
		t.Fatalf("evidence_mode = %q, want mcp", repo.lastMode)
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
