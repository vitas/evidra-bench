package benchsvc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleToolServerMatrixReport_ReturnsJSON(t *testing.T) {
	t.Parallel()

	mux := setupMux(matrixReportRepo(), ServiceConfig{PublicTenant: "bench-public"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/bench/reports/tool-server-matrix?model=sonnet&report_id=public-report&tool_servers=flux159-mcp-server-kubernetes,containers-kubernetes-mcp-server&tool_server_versions=1.0.0,2.0.0&scenarios=s1,s2,s3,s4", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var report ToolServerMatrixReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if report.ReportID != "public-report" {
		t.Fatalf("report_id = %q, want public-report", report.ReportID)
	}
	if len(report.Arms) != 3 {
		t.Fatalf("arms = %+v, want baseline plus two candidates", report.Arms)
	}
}

func TestHandleToolServerMatrixReport_ReturnsMarkdown(t *testing.T) {
	t.Parallel()

	mux := setupMux(matrixReportRepo(), ServiceConfig{PublicTenant: "bench-public"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/bench/reports/tool-server-matrix?model=sonnet&report_id=public-report&tool_servers=flux159-mcp-server-kubernetes,containers-kubernetes-mcp-server&tool_server_versions=1.0.0,2.0.0&scenarios=s1,s2,s3,s4&format=markdown", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/markdown") {
		t.Fatalf("content-type = %q, want text/markdown", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), "# Kubernetes MCP Server Readiness Report") {
		t.Fatalf("markdown body missing title:\n%s", rec.Body.String())
	}
}

func TestHandleToolServerMatrixReport_RequiresInputs(t *testing.T) {
	t.Parallel()

	mux := setupMux(matrixReportRepo(), ServiceConfig{PublicTenant: "bench-public"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/bench/reports/tool-server-matrix?model=sonnet&report_id=public-report", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}
