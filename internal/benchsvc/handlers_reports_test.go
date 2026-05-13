package benchsvc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	bench "github.com/vitas/evidra-bench/pkg/bench"
)

func TestHandleToolServerReport_ReturnsJSON(t *testing.T) {
	t.Parallel()

	repo := reportHandlerRepo()
	mux := setupMux(repo, ServiceConfig{PublicTenant: "bench-public"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/bench/reports/tool-server?model=sonnet&tool_server=kubernetes-mcp&tool_server_version=1.2.3&scenarios=s1", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content-type = %q, want application/json", rec.Header().Get("Content-Type"))
	}
	var report ToolServerReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if report.Configuration.Model != "sonnet" || report.Configuration.ToolServer != "kubernetes-mcp" {
		t.Fatalf("configuration = %+v, want selected filters", report.Configuration)
	}
	if repo.lastTenant != "bench-public" {
		t.Fatalf("tenant = %q, want bench-public", repo.lastTenant)
	}
}

func TestHandleToolServerReport_ReturnsMarkdown(t *testing.T) {
	t.Parallel()

	mux := setupMux(reportHandlerRepo(), ServiceConfig{PublicTenant: "bench-public"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/bench/reports/tool-server?model=sonnet&tool_server=kubernetes-mcp&format=markdown", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/markdown") {
		t.Fatalf("content-type = %q, want text/markdown", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), "# Evidra Bench Tool Server Report") {
		t.Fatalf("markdown body missing title:\n%s", rec.Body.String())
	}
}

func TestHandleToolServerReport_UsesEmptyArraysForMissingOptionalSections(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	repo := &handlerRepo{
		scenarios: []bench.ScenarioSummary{
			{ID: "s1", Title: "Scenario 1", Category: "kubernetes", Level: "L2"},
		},
		runs: []bench.RunRecord{
			reportRun("baseline-s1", "s1", "", "", true, start),
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "bench-public"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/bench/reports/tool-server?model=sonnet&tool_server=kubernetes-mcp&scenarios=s1", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	for _, field := range []string{"autopsies", "evidence_links"} {
		if string(body[field]) != "[]" {
			t.Fatalf("%s = %s, want []", field, body[field])
		}
	}
}

func TestHandleToolServerReport_FiltersByReportID(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	publicBaseline := reportRun("baseline-public", "s1", "", "", true, start)
	publicBaseline.MetadataJSON = `{"report_id":"public-report"}`
	publicCandidate := reportRun("candidate-public", "s1", "kubernetes-mcp", "1.2.3", true, start.Add(time.Minute))
	publicCandidate.MetadataJSON = `{"report_id":"public-report"}`
	otherBaseline := reportRun("baseline-other", "s1", "", "", false, start.Add(2*time.Minute))
	otherBaseline.MetadataJSON = `{"report_id":"other-report"}`
	otherCandidate := reportRun("candidate-other", "s1", "kubernetes-mcp", "1.2.3", false, start.Add(3*time.Minute))
	otherCandidate.MetadataJSON = `{"report_id":"other-report"}`

	repo := &handlerRepo{
		scenarios: []bench.ScenarioSummary{
			{ID: "s1", Title: "Scenario 1", Category: "kubernetes", Level: "L2"},
		},
		runs: []bench.RunRecord{publicBaseline, publicCandidate, otherBaseline, otherCandidate},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "bench-public"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/bench/reports/tool-server?model=sonnet&tool_server=kubernetes-mcp&tool_server_version=1.2.3&scenarios=s1&report_id=public-report", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var report ToolServerReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if report.Configuration.ReportID != "public-report" {
		t.Fatalf("configuration.report_id = %q, want public-report", report.Configuration.ReportID)
	}
	if report.Summary.SafePass != 1 || report.Summary.Fail != 0 {
		t.Fatalf("summary = %+v, want one safe pass from public report only", report.Summary)
	}
	if len(report.Scenarios) != 1 || report.Scenarios[0].CandidateRunID != "candidate-public" {
		t.Fatalf("scenarios = %+v, want candidate-public only", report.Scenarios)
	}
}

func TestHandleToolServerReport_RequiresModelAndToolServer(t *testing.T) {
	t.Parallel()

	mux := setupMux(&handlerRepo{}, ServiceConfig{PublicTenant: "bench-public"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/bench/reports/tool-server?model=sonnet", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func reportHandlerRepo() *handlerRepo {
	start := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	return &handlerRepo{
		scenarios: []bench.ScenarioSummary{
			{ID: "s1", Title: "Scenario 1", Category: "kubernetes", Level: "L2"},
		},
		runs: []bench.RunRecord{
			reportRun("baseline-s1", "s1", "", "", true, start),
			reportRun("candidate-s1", "s1", "kubernetes-mcp", "1.2.3", true, start.Add(time.Minute)),
		},
	}
}
