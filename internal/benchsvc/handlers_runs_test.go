package benchsvc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	bench "samebits.com/evidra-infra-bench/pkg/bench"
)

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
	req := httptest.NewRequest("GET", "/v1/bench/runs?model=sonnet&scenario=broken-deployment&scenarios=s1,s2&evidence_mode=mcp&tool_server=kubernetes-mcp&tool_server_version=1.2.3&limit=10&offset=5", nil)
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
	if !reflect.DeepEqual(f.ScenarioIDs, []string{"s1", "s2"}) {
		t.Errorf("ScenarioIDs = %#v, want s1,s2", f.ScenarioIDs)
	}
	if f.EvidenceMode != "mcp" {
		t.Errorf("EvidenceMode = %q, want mcp", f.EvidenceMode)
	}
	if f.ToolServer != "kubernetes-mcp" {
		t.Errorf("ToolServer = %q, want kubernetes-mcp", f.ToolServer)
	}
	if f.ToolServerVersion != "1.2.3" {
		t.Errorf("ToolServerVersion = %q, want 1.2.3", f.ToolServerVersion)
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
		{ID: "mcp-1", ScenarioID: "s3", Model: "sonnet", EvidenceMode: "mcp"},
		{ID: "mcp-2", ScenarioID: "s4", Model: "sonnet", EvidenceMode: "mcp"},
	}

	tests := []struct {
		name    string
		mode    string
		wantIDs []string
	}{
		{name: "baseline only", mode: "none", wantIDs: []string{"baseline-1", "baseline-2"}},
		{name: "mcp", mode: "mcp", wantIDs: []string{"mcp-1", "mcp-2"}},
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
