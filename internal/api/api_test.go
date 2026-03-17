package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"samebits.com/evidra-infra-bench/pkg/store"
)

// mockStore implements store.BenchStore for testing.
type mockStore struct {
	runs      []store.RunRecord
	scenarios []store.ScenarioSummary
	lastStats store.RunFilters
}

func (m *mockStore) ListRuns(_ context.Context, f store.RunFilters) ([]store.RunRecord, int, error) {
	var filtered []store.RunRecord
	for _, r := range m.runs {
		if f.ScenarioID != "" && r.ScenarioID != f.ScenarioID {
			continue
		}
		if f.Model != "" && r.Model != f.Model {
			continue
		}
		if f.Provider != "" && r.Provider != f.Provider {
			continue
		}
		filtered = append(filtered, r)
	}
	total := len(filtered)
	if f.Offset > 0 && f.Offset < len(filtered) {
		filtered = filtered[f.Offset:]
	}
	if f.Limit > 0 && f.Limit < len(filtered) {
		filtered = filtered[:f.Limit]
	}
	return filtered, total, nil
}

func (m *mockStore) GetRun(_ context.Context, id string) (*store.RunRecord, error) {
	for _, r := range m.runs {
		if r.ID == id {
			return &r, nil
		}
	}
	return nil, &notFoundError{id: id}
}

func (m *mockStore) Catalog(_ context.Context) (*store.RunCatalog, error) {
	models := map[string]struct{}{}
	providers := map[string]struct{}{}
	for _, r := range m.runs {
		if r.Model != "" {
			models[r.Model] = struct{}{}
		}
		if r.Provider != "" {
			providers[r.Provider] = struct{}{}
		}
	}
	modelList := make([]string, 0, len(models))
	for model := range models {
		modelList = append(modelList, model)
	}
	providerList := make([]string, 0, len(providers))
	for provider := range providers {
		providerList = append(providerList, provider)
	}
	slices.Sort(modelList)
	slices.Sort(providerList)
	return &store.RunCatalog{Models: modelList, Providers: providerList}, nil
}

func (m *mockStore) CompareRuns(_ context.Context, idA, idB string) (*store.RunComparison, error) {
	var a, b *store.RunRecord
	for i := range m.runs {
		if m.runs[i].ID == idA {
			a = &m.runs[i]
		}
		if m.runs[i].ID == idB {
			b = &m.runs[i]
		}
	}
	if a == nil || b == nil {
		return nil, &notFoundError{id: idA + "/" + idB}
	}
	return &store.RunComparison{RunA: *a, RunB: *b}, nil
}

func (m *mockStore) ModelMatrix(_ context.Context, models, scenarios []string) (*store.ModelMatrix, error) {
	return &store.ModelMatrix{
		Models:    models,
		Scenarios: []string{"s1"},
		Cells:     map[string]map[string]store.ModelMatrixCell{},
	}, nil
}

func (m *mockStore) FilteredStats(_ context.Context, f store.RunFilters) (*store.StatsResult, error) {
	m.lastStats = f
	return &store.StatsResult{
		TotalRuns: len(m.runs),
		PassCount: len(m.runs),
	}, nil
}

func (m *mockStore) ListScenarios(_ context.Context) ([]store.ScenarioSummary, error) {
	return m.scenarios, nil
}

func (m *mockStore) SignalSummary(_ context.Context, _ store.RunFilters) (*store.SignalAggregation, error) {
	return &store.SignalAggregation{
		TotalRuns:         2,
		RunsWithScorecard: 1,
		Signals:           map[string]store.SignalCount{"protocol_violation": {Total: 1, RunCount: 1}},
		AvgScore:          87.5,
	}, nil
}

type notFoundError struct{ id string }

func (e *notFoundError) Error() string { return "not found: " + e.id }

func newTestServer() (*Server, *mockStore) {
	ms := &mockStore{
		runs: []store.RunRecord{
			{
				ID:         "run-1",
				ScenarioID: "broken-deployment",
				Model:      "sonnet",
				Provider:   "claude",
				Passed:     true,
				Duration:   45.2,
				Turns:      11,
				CreatedAt:  time.Now().UTC(),
			},
			{
				ID:         "run-2",
				ScenarioID: "broken-deployment",
				Model:      "haiku",
				Provider:   "claude",
				Passed:     false,
				Duration:   30.1,
				Turns:      8,
				CreatedAt:  time.Now().UTC(),
			},
		},
	}
	srv := NewServer(ms, nil, "", "test")
	return srv, ms
}

func TestHealthz(t *testing.T) {
	t.Parallel()
	srv, _ := newTestServer()
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestListRuns(t *testing.T) {
	t.Parallel()
	srv, _ := newTestServer()
	req := httptest.NewRequest("GET", "/v1/bench/runs", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp listResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 2 {
		t.Fatalf("expected total=2, got %d", resp.Total)
	}
}

func TestListRuns_FilterByModel(t *testing.T) {
	t.Parallel()
	srv, _ := newTestServer()
	req := httptest.NewRequest("GET", "/v1/bench/runs?model=sonnet", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp listResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 1 {
		t.Fatalf("expected total=1, got %d", resp.Total)
	}
}

func TestGetRun(t *testing.T) {
	t.Parallel()
	srv, _ := newTestServer()
	req := httptest.NewRequest("GET", "/v1/bench/runs/run-1", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var run store.RunRecord
	json.NewDecoder(w.Body).Decode(&run)
	if run.Model != "sonnet" {
		t.Fatalf("expected sonnet, got %s", run.Model)
	}
}

func TestGetRun_NotFound(t *testing.T) {
	t.Parallel()
	srv, _ := newTestServer()
	req := httptest.NewRequest("GET", "/v1/bench/runs/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestCompareRuns(t *testing.T) {
	t.Parallel()
	srv, _ := newTestServer()
	req := httptest.NewRequest("GET", "/v1/bench/compare/runs?a=run-1&b=run-2", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var cmp store.RunComparison
	json.NewDecoder(w.Body).Decode(&cmp)
	if cmp.RunA.ID != "run-1" || cmp.RunB.ID != "run-2" {
		t.Fatal("wrong run IDs in comparison")
	}
}

func TestCompareRuns_MissingParams(t *testing.T) {
	t.Parallel()
	srv, _ := newTestServer()
	req := httptest.NewRequest("GET", "/v1/bench/compare/runs", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCompareModels(t *testing.T) {
	t.Parallel()
	srv, _ := newTestServer()
	req := httptest.NewRequest("GET", "/v1/bench/compare/models?models=sonnet,haiku", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCompareModels_MissingParam(t *testing.T) {
	t.Parallel()
	srv, _ := newTestServer()
	req := httptest.NewRequest("GET", "/v1/bench/compare/models", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCatalog(t *testing.T) {
	t.Parallel()
	srv, _ := newTestServer()
	req := httptest.NewRequest("GET", "/v1/bench/catalog", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Models    []string `json:"models"`
		Providers []string `json:"providers"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	if len(resp.Models) != 2 || resp.Models[0] != "haiku" || resp.Models[1] != "sonnet" {
		t.Fatalf("models = %#v, want [haiku sonnet]", resp.Models)
	}
	if len(resp.Providers) != 1 || resp.Providers[0] != "claude" {
		t.Fatalf("providers = %#v, want [claude]", resp.Providers)
	}
}

func TestStats(t *testing.T) {
	t.Parallel()
	srv, _ := newTestServer()
	req := httptest.NewRequest("GET", "/v1/bench/stats", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var st store.StatsResult
	json.NewDecoder(w.Body).Decode(&st)
	if st.TotalRuns != 2 {
		t.Fatalf("expected 2 total, got %d", st.TotalRuns)
	}
}

func TestStats_SinceFilter(t *testing.T) {
	t.Parallel()
	srv, ms := newTestServer()
	req := httptest.NewRequest("GET", "/v1/bench/stats?since=2026-03-17T10:00:00Z", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ms.lastStats.Since != "2026-03-17T10:00:00Z" {
		t.Fatalf("since filter = %q, want %q", ms.lastStats.Since, "2026-03-17T10:00:00Z")
	}
}

func TestExecute_NoExecutor(t *testing.T) {
	t.Parallel()
	srv, _ := newTestServer() // exec is nil
	body := `{"scenario_id":"broken-deployment","model":"sonnet","provider":"claude"}`
	req := httptest.NewRequest("POST", "/v1/bench/execute", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestExecuteStatus_NoExecutor(t *testing.T) {
	t.Parallel()
	srv, _ := newTestServer()
	req := httptest.NewRequest("GET", "/v1/bench/execute/job-1/status", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestSignals(t *testing.T) {
	t.Parallel()
	srv, _ := newTestServer()
	req := httptest.NewRequest("GET", "/v1/bench/signals", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCORS(t *testing.T) {
	t.Parallel()
	srv, _ := newTestServer()
	req := httptest.NewRequest("OPTIONS", "/v1/bench/runs", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("missing CORS header")
	}
}
