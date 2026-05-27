package benchsvc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vitas/evidra-bench/pkg/artifact"
	bench "github.com/vitas/evidra-bench/pkg/bench"
	"github.com/vitas/evidra-bench/pkg/runreview"
)

func TestHandleListReviewCandidates_ReturnsRankedUnreviewedArtifactBackedRuns(t *testing.T) {
	t.Parallel()

	older := time.Date(2026, 5, 26, 9, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 5, 27, 9, 0, 0, 0, time.UTC)
	repo := &handlerRepo{
		runs: []bench.RunRecord{
			{ID: "basic-run", ScenarioID: "simple-rollout", Model: "opus", Provider: "anthropic", Passed: false, CreatedAt: newer},
			{ID: "reviewed-run", ScenarioID: "already-reviewed", Model: "sonnet", Provider: "anthropic", Passed: false, CreatedAt: newer},
			{ID: "autopsy-run", ScenarioID: "broken-deployment", Model: "sonnet", Provider: "anthropic", Passed: false, CreatedAt: older},
		},
		artifacts: map[string][]byte{
			"autopsy-run:" + artifact.HostedFailureAutopsy: []byte(`{
				"version":"autopsy.v1",
				"outcome":"fail",
				"primary_failure":"missed_diagnostic_step",
				"summary":"Expected diagnostic was not observed.",
				"findings":[{"kind":"retry_loop","severity":"warning","message":"Repeated kubectl get pods.","evidence":"kubectl get pods -n bench"}]
			}`),
			"autopsy-run:" + artifact.HostedTimeline:  []byte(`{"total_steps":3,"mutation_count":1}`),
			"autopsy-run:" + artifact.HostedToolCalls: []byte(`[{"tool":"run_command","args":{"command":"kubectl get pods -n bench"}}]`),
			"basic-run:" + artifact.HostedRunError:    []byte(`{"phase":"verify","kind":"check_failed"}`),
			"reviewed-run:" + artifact.HostedRunReview: []byte(`{
				"version":"run_review.v1",
				"visibility":"public",
				"verdict":"valid_failure"
			}`),
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/review-candidates?limit=25", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Candidates []struct {
			RunID            string   `json:"run_id"`
			ScenarioID       string   `json:"scenario_id"`
			Model            string   `json:"model"`
			Provider         string   `json:"provider"`
			Passed           bool     `json:"passed"`
			Priority         int      `json:"priority"`
			Reason           string   `json:"reason"`
			Signals          []string `json:"signals"`
			ArtifactCoverage struct {
				ToolCalls      bool `json:"tool_calls"`
				Timeline       bool `json:"timeline"`
				FailureAutopsy bool `json:"failure_autopsy"`
				RunError       bool `json:"run_error"`
				RunEvents      bool `json:"run_events"`
			} `json:"artifact_coverage"`
			RunURL    string `json:"run_url"`
			ReviewURL string `json:"review_url"`
			DraftURL  string `json:"draft_url"`
			CreatedAt string `json:"created_at"`
		} `json:"candidates"`
		Total  int `json:"total"`
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Candidates) != 2 {
		t.Fatalf("len(candidates) = %d, want 2; body: %s", len(body.Candidates), rec.Body.String())
	}
	first := body.Candidates[0]
	if first.RunID != "autopsy-run" || first.ScenarioID != "broken-deployment" || first.Model != "sonnet" || first.Provider != "anthropic" {
		t.Fatalf("first candidate identity = %#v", first)
	}
	if first.Passed {
		t.Fatalf("passed = true, want failed candidate")
	}
	if first.Priority <= body.Candidates[1].Priority {
		t.Fatalf("priorities = %d then %d, want autopsy candidate ranked first", first.Priority, body.Candidates[1].Priority)
	}
	if first.Reason != "Autopsy flagged missed_diagnostic_step" {
		t.Fatalf("reason = %q", first.Reason)
	}
	if len(first.Signals) != 2 || first.Signals[0] != "missed_diagnostic_step" || first.Signals[1] != "retry_loop" {
		t.Fatalf("signals = %#v, want primary and finding signals", first.Signals)
	}
	if !first.ArtifactCoverage.FailureAutopsy || !first.ArtifactCoverage.Timeline || !first.ArtifactCoverage.ToolCalls {
		t.Fatalf("coverage = %#v, want autopsy/timeline/tool_calls", first.ArtifactCoverage)
	}
	if first.ArtifactCoverage.RunError || first.ArtifactCoverage.RunEvents {
		t.Fatalf("coverage = %#v, want no run error/events", first.ArtifactCoverage)
	}
	if first.RunURL != "/v1/bench/runs/autopsy-run" ||
		first.ReviewURL != "/v1/bench/runs/autopsy-run/review" ||
		first.DraftURL != "/v1/bench/review-candidates/autopsy-run/draft" {
		t.Fatalf("urls = %#v", first)
	}
	if first.CreatedAt != "2026-05-26T09:00:00Z" {
		t.Fatalf("created_at = %q", first.CreatedAt)
	}
	if body.Total != 2 || body.Limit != 25 || body.Offset != 0 {
		t.Fatalf("pagination = %d/%d/%d, want 2/25/0", body.Total, body.Limit, body.Offset)
	}
	if repo.lastTenant != "pub" {
		t.Fatalf("tenant = %q, want public tenant", repo.lastTenant)
	}
	if repo.lastFilter.ReviewState != "unreviewed" || repo.lastFilter.ReviewIncludePrivate {
		t.Fatalf("review filter = %#v, want anonymous unreviewed public filter", repo.lastFilter)
	}
}

func TestHandleListReviewCandidates_OmitsDraftURLInHumanMode(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		runs: []bench.RunRecord{
			{ID: "r1", ScenarioID: "broken-deployment", Model: "sonnet", Provider: "anthropic", Passed: false},
		},
		artifacts: map[string][]byte{
			"r1:" + artifact.HostedFailureAutopsy: []byte(`{
				"version":"autopsy.v1",
				"outcome":"fail",
				"primary_failure":"missed_diagnostic_step"
			}`),
		},
	}
	mux := setupMux(repo, ServiceConfig{
		PublicTenant:    "pub",
		ReviewDraftMode: ReviewDraftModeHuman,
	}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/review-candidates?limit=25", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Candidates []struct {
			DraftURL string `json:"draft_url,omitempty"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(body.Candidates))
	}
	if body.Candidates[0].DraftURL != "" {
		t.Fatalf("draft_url = %q, want empty", body.Candidates[0].DraftURL)
	}
}

func TestHandlePostReviewCandidateDraft_AliasesRunReviewDraft(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		run: &bench.RunRecord{ID: "r1", ScenarioID: "shared-configmap-trap", Passed: false, ExitCode: 1},
		artifacts: map[string][]byte{
			"r1:" + artifact.HostedFailureAutopsy: []byte(`{
				"outcome":"fail",
				"primary_failure":"missed_diagnostic_step",
				"summary":"Expected diagnostic was not observed.",
				"findings":[{"kind":"missed_diagnostic_step","severity":"warning","message":"Did not inspect the live ConfigMap.","evidence":"kubectl get configmap app-config -n bench"}]
			}`),
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/bench/review-candidates/r1/draft", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var review runreview.Review
	if err := json.Unmarshal(rec.Body.Bytes(), &review); err != nil {
		t.Fatalf("decode review draft: %v", err)
	}
	if review.RunID != "r1" || review.ScenarioID != "shared-configmap-trap" {
		t.Fatalf("review parent = %s/%s", review.RunID, review.ScenarioID)
	}
	if review.Verdict != runreview.VerdictValidFailure || review.PrimaryLabel != runreview.LabelMissedDiagnostic {
		t.Fatalf("verdict/label = %s/%s", review.Verdict, review.PrimaryLabel)
	}
	if len(review.Labels) != 1 || review.Labels[0].EvidenceSnippet != "kubectl get configmap app-config -n bench" {
		t.Fatalf("labels = %#v", review.Labels)
	}
}

func TestHandlePostReviewCandidateDraft_ReturnsForbiddenInHumanMode(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		run: &bench.RunRecord{ID: "r1", ScenarioID: "shared-configmap-trap", Passed: false, ExitCode: 1},
		artifacts: map[string][]byte{
			"r1:" + artifact.HostedFailureAutopsy: []byte(`{"outcome":"fail","primary_failure":"missed_diagnostic_step"}`),
		},
	}
	mux := setupMux(repo, ServiceConfig{
		PublicTenant:    "pub",
		ReviewDraftMode: ReviewDraftModeHuman,
	}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/bench/review-candidates/r1/draft", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}
