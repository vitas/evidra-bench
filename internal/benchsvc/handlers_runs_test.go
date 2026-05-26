package benchsvc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/vitas/evidra-bench/internal/auth"
	"github.com/vitas/evidra-bench/pkg/artifact"
	bench "github.com/vitas/evidra-bench/pkg/bench"
	"github.com/vitas/evidra-bench/pkg/runreview"
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

func TestHandleListRuns_AttachesPublicReviewSummary(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		runs: []bench.RunRecord{
			{ID: "r1", ScenarioID: "shared-configmap-trap", Model: "sonnet", Passed: true},
		},
		artifacts: map[string][]byte{
			"r1:" + artifact.HostedRunReview: []byte(`{
				"version":"run_review.v1",
				"visibility":"public",
				"verdict":"unsafe_pass",
				"primary_label":"unsafe_action",
				"labels":[
					{"kind":"unsafe_action","severity":"warning","note":"unsafe","evidence_snippet":"pods_delete Pod/web"},
					{"kind":"wrong_scope","severity":"error","note":"wrong namespace","evidence_snippet":"namespace prod"}
				]
			}`),
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Items []bench.RunRecord `json:"runs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(body.Items))
	}
	summary := body.Items[0].ReviewSummary
	if summary == nil {
		t.Fatal("review_summary missing")
		return
	}
	if summary.Verdict != runreview.VerdictUnsafePass || summary.PrimaryLabel != runreview.LabelUnsafeAction {
		t.Fatalf("summary verdict/label = %#v", summary)
	}
	if summary.Visibility != runreview.VisibilityPublic || summary.LabelCount != 2 || summary.MaxSeverity != runreview.SeverityError {
		t.Fatalf("summary metadata = %#v", summary)
	}
}

func TestHandleListRuns_HidesPrivateReviewSummaryFromAnonymousRead(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		runs: []bench.RunRecord{
			{ID: "r1", ScenarioID: "shared-configmap-trap", Model: "sonnet", Passed: true},
		},
		artifacts: map[string][]byte{
			"r1:" + artifact.HostedRunReview: []byte(`{"version":"run_review.v1","visibility":"private","verdict":"needs_review"}`),
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Items []bench.RunRecord `json:"runs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Items[0].ReviewSummary != nil {
		t.Fatalf("review_summary = %#v, want hidden", body.Items[0].ReviewSummary)
	}
}

func TestHandleListRuns_AttachesPrivateReviewSummaryWithSessionCookie(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		runs: []bench.RunRecord{
			{ID: "r1", ScenarioID: "shared-configmap-trap", Model: "sonnet", Passed: true},
		},
		artifacts: map[string][]byte{
			"r1:" + artifact.HostedRunReview: []byte(`{"version":"run_review.v1","visibility":"private","verdict":"unsafe_pass","labels":[{"kind":"unsafe_action","severity":"warning","note":"unsafe","evidence_snippet":"pods_delete Pod/web"}]}`),
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "signed-session"})
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Items []bench.RunRecord `json:"runs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Items[0].ReviewSummary == nil {
		t.Fatal("review_summary missing")
		return
	}
	if body.Items[0].ReviewSummary.Visibility != runreview.VisibilityPrivate {
		t.Fatalf("visibility = %q, want private", body.Items[0].ReviewSummary.Visibility)
	}
}

func TestHandleListRuns_ParsesFilters(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{runsTotal: 0}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-b")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs?model=sonnet&scenario=broken-deployment&scenarios=s1,s2&tool_server=kubernetes-mcp&tool_server_version=1.2.3&skill_id=k8s-admin&skill_version=2026-05-13&report_id=public-report&tool_server_unset=true&limit=10&offset=5", nil)
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
	if f.ToolServer != "kubernetes-mcp" {
		t.Errorf("ToolServer = %q, want kubernetes-mcp", f.ToolServer)
	}
	if !f.ToolServerUnset {
		t.Errorf("ToolServerUnset = false, want true")
	}
	if f.ToolServerVersion != "1.2.3" {
		t.Errorf("ToolServerVersion = %q, want 1.2.3", f.ToolServerVersion)
	}
	if f.SkillID != "k8s-admin" {
		t.Errorf("SkillID = %q, want k8s-admin", f.SkillID)
	}
	if f.SkillVersion != "2026-05-13" {
		t.Errorf("SkillVersion = %q, want 2026-05-13", f.SkillVersion)
	}
	if f.ReportID != "public-report" {
		t.Errorf("ReportID = %q, want public-report", f.ReportID)
	}
	if f.Limit != 10 {
		t.Errorf("Limit = %d, want 10", f.Limit)
	}
	if f.Offset != 5 {
		t.Errorf("Offset = %d, want 5", f.Offset)
	}
}

func TestHandleListRuns_ParsesReviewFilters(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{runsTotal: 0}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-b")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs?review=reviewed&review_verdict=unsafe_pass&review_severity=critical&review_visibility=private&reviewer=Evidra", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	f := repo.lastFilter
	if f.ReviewState != "reviewed" {
		t.Errorf("ReviewState = %q, want reviewed", f.ReviewState)
	}
	if f.ReviewVerdict != runreview.VerdictUnsafePass {
		t.Errorf("ReviewVerdict = %q, want unsafe_pass", f.ReviewVerdict)
	}
	if f.ReviewSeverity != runreview.SeverityCritical {
		t.Errorf("ReviewSeverity = %q, want critical", f.ReviewSeverity)
	}
	if f.ReviewVisibility != runreview.VisibilityPrivate {
		t.Errorf("ReviewVisibility = %q, want private", f.ReviewVisibility)
	}
	if f.Reviewer != "Evidra" {
		t.Errorf("Reviewer = %q, want Evidra", f.Reviewer)
	}
	if !f.ReviewIncludePrivate {
		t.Error("ReviewIncludePrivate = false, want true for authenticated read")
	}
}

func TestHandleListRuns_ToolServerUnsetFiltersItems(t *testing.T) {
	t.Parallel()

	sharedRuns := []bench.RunRecord{
		{ID: "baseline-1", ScenarioID: "s1", Model: "sonnet"},
		{ID: "baseline-2", ScenarioID: "s2", Model: "sonnet"},
		{ID: "mcp-1", ScenarioID: "s3", Model: "sonnet", ToolServer: "kubernetes-mcp"},
		{ID: "mcp-2", ScenarioID: "s4", Model: "sonnet", ToolServer: "kubernetes-mcp"},
	}
	repo := &handlerRepo{runs: sharedRuns}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs?tool_server_unset=true", nil)
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
	if body.Total != 2 {
		t.Fatalf("total = %d, want 2", body.Total)
	}
	for i, wantID := range []string{"baseline-1", "baseline-2"} {
		if body.Items[i].ID != wantID {
			t.Fatalf("items[%d].ID = %q, want %q", i, body.Items[i].ID, wantID)
		}
	}
}

func TestHandleListRuns_FiltersByReportID(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		runs: []bench.RunRecord{
			{ID: "public-run", ScenarioID: "s1", Model: "sonnet", MetadataJSON: `{"report_id":"public-report"}`},
			{ID: "other-run", ScenarioID: "s1", Model: "sonnet", MetadataJSON: `{"report_id":"other-report"}`},
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs?report_id=public-report", nil)
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
	if body.Total != 1 || len(body.Items) != 1 {
		t.Fatalf("runs = %+v total=%d, want one public-report run", body.Items, body.Total)
	}
	if body.Items[0].ID != "public-run" {
		t.Fatalf("run ID = %q, want public-run", body.Items[0].ID)
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

func TestHandleGetRun_AttachesPrivateReviewSummaryForAuthenticatedRead(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		run: &bench.RunRecord{ID: "run-42", ScenarioID: "s1", Model: "sonnet", Passed: false},
		artifacts: map[string][]byte{
			"run-42:" + artifact.HostedRunReview: []byte(`{"version":"run_review.v1","visibility":"private","verdict":"valid_failure","labels":[{"kind":"missed_diagnostic","severity":"critical","note":"missed","evidence_snippet":"no describe"}]}`),
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs/run-42", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var run bench.RunRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &run); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if run.ReviewSummary == nil {
		t.Fatal("review_summary missing")
		return
	}
	if run.ReviewSummary.Verdict != runreview.VerdictValidFailure || run.ReviewSummary.MaxSeverity != runreview.SeverityCritical {
		t.Fatalf("summary = %#v", run.ReviewSummary)
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
