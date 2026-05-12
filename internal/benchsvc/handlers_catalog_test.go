package benchsvc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	bench "samebits.com/evidra-infra-bench/pkg/bench"
)

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
		{ID: "mcp-1", ScenarioID: "s3", Model: "sonnet", EvidenceMode: "mcp", Passed: true},
		{ID: "mcp-2", ScenarioID: "s4", Model: "sonnet", EvidenceMode: "mcp", Passed: false},
	}

	tests := []struct {
		name      string
		mode      string
		wantTotal int
		wantPass  int
		wantFail  int
	}{
		{name: "baseline only", mode: "none", wantTotal: 2, wantPass: 1, wantFail: 1},
		{name: "mcp", mode: "mcp", wantTotal: 2, wantPass: 1, wantFail: 1},
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
			Models:             []string{"sonnet", "opus"},
			Providers:          []string{"anthropic", "bifrost"},
			ToolServers:        []string{"kubernetes-mcp", "legacy-mcp"},
			ToolServerVersions: []string{"1.2.3", "1.2.4"},
			ToolServerVersionsByServer: map[string][]string{
				"kubernetes-mcp": {"1.2.3"},
				"legacy-mcp":     {"1.2.4"},
			},
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
	if len(body.ToolServers) != 2 {
		t.Fatalf("len(ToolServers) = %d, want 2", len(body.ToolServers))
	}
	if len(body.ToolServerVersions) != 2 {
		t.Fatalf("len(ToolServerVersions) = %d, want 2", len(body.ToolServerVersions))
	}
	if got := body.ToolServerVersionsByServer["kubernetes-mcp"]; len(got) != 1 || got[0] != "1.2.3" {
		t.Fatalf("ToolServerVersionsByServer[kubernetes-mcp] = %v, want [1.2.3]", got)
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
