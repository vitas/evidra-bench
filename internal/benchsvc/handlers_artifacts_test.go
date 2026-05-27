package benchsvc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vitas/evidra-bench/pkg/artifact"
	bench "github.com/vitas/evidra-bench/pkg/bench"
	"github.com/vitas/evidra-bench/pkg/runreview"
)

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
		artifacts: map[string][]byte{
			"r1:tool_calls": data,
		},
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

func TestHandleGetTimeline_ReturnsStoredTimelineWhenPresent(t *testing.T) {
	t.Parallel()

	stored := []byte(`{"total_steps":7,"mutation_count":2,"phase_count":{"act":2}}`)
	repo := &handlerRepo{
		artifacts: map[string][]byte{
			"r1:timeline": stored,
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs/r1/timeline", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if repo.lastArtifactType != "timeline" {
		t.Fatalf("artifact type = %q, want timeline", repo.lastArtifactType)
	}
	var tl bench.Timeline
	if err := json.Unmarshal(rec.Body.Bytes(), &tl); err != nil {
		t.Fatalf("decode timeline: %v", err)
	}
	if tl.TotalSteps != 7 {
		t.Fatalf("TotalSteps = %d, want 7", tl.TotalSteps)
	}
	if tl.MutationCount != 2 {
		t.Fatalf("MutationCount = %d, want 2", tl.MutationCount)
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

func TestHandleGetAutopsy_ReturnsJSON(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		artifact: []byte(`{"outcome":"fail","primary_failure":"premature_success"}`),
		artCT:    "application/json",
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs/r1/autopsy", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if repo.lastArtifactType != "failure_autopsy" {
		t.Fatalf("artifact type = %q, want failure_autopsy", repo.lastArtifactType)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	var body struct {
		PrimaryFailure string `json:"primary_failure"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.PrimaryFailure != "premature_success" {
		t.Fatalf("primary_failure = %q, want premature_success", body.PrimaryFailure)
	}
}

func TestHandleGetAutopsy_404WhenMissing(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{artErr: ErrNotFound}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs/r1/autopsy", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if repo.lastArtifactType != "failure_autopsy" {
		t.Fatalf("artifact type = %q, want failure_autopsy", repo.lastArtifactType)
	}
}

func TestHandleGetRunErrorAndEvents_ReturnsJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		path         string
		artifactType string
		body         []byte
	}{
		{
			name:         "run error",
			path:         "/v1/bench/runs/r1/run-error",
			artifactType: "run_error",
			body:         []byte(`{"phase":"agent_run","kind":"adapter_error"}`),
		},
		{
			name:         "run events",
			path:         "/v1/bench/runs/r1/run-events",
			artifactType: "run_events",
			body:         []byte(`[{"phase":"run","status":"started"},{"phase":"agent_run","status":"failed"}]`),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := &handlerRepo{
				artifacts: map[string][]byte{
					"r1:" + tt.artifactType: tt.body,
				},
			}
			mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", tt.path, nil)
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
			}
			if repo.lastArtifactType != tt.artifactType {
				t.Fatalf("artifact type = %q, want %s", repo.lastArtifactType, tt.artifactType)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Fatalf("Content-Type = %q, want application/json", ct)
			}
			if !json.Valid(rec.Body.Bytes()) {
				t.Fatalf("body is not JSON: %s", rec.Body.String())
			}
		})
	}
}

func TestHandleGetRunReview_ReturnsPublicReview(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		artifacts: map[string][]byte{
			"r1:" + artifact.HostedRunReview: []byte(`{"version":"run_review.v1","visibility":"public","verdict":"unsafe_pass","labels":[{"kind":"unsafe_action","severity":"warning","note":"Unsafe shortcut.","evidence_snippet":"pods_delete Pod/web"}]}`),
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs/r1/review", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if repo.lastArtifactType != artifact.HostedRunReview {
		t.Fatalf("artifact type = %q, want %s", repo.lastArtifactType, artifact.HostedRunReview)
	}
	var body runreview.Review
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode review: %v", err)
	}
	if body.Verdict != runreview.VerdictUnsafePass {
		t.Fatalf("verdict = %q, want unsafe_pass", body.Verdict)
	}
}

func TestHandleGetRunReview_HidesPrivateReviewFromAnonymousRead(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		artifacts: map[string][]byte{
			"r1:" + artifact.HostedRunReview: []byte(`{"version":"run_review.v1","visibility":"private","verdict":"needs_review"}`),
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs/r1/review", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestHandlePostRunReviewDraft_UsesAutopsyEvidence(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		run: &bench.RunRecord{
			ID:         "r1",
			ScenarioID: "shared-configmap-trap",
			Passed:     false,
			ExitCode:   1,
		},
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
	req := httptest.NewRequest("POST", "/v1/bench/runs/r1/review-draft", nil)
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
	if review.Visibility != runreview.VisibilityPrivate {
		t.Fatalf("visibility = %q, want private", review.Visibility)
	}
	if len(review.Labels) != 1 {
		t.Fatalf("labels = %d, want 1", len(review.Labels))
	}
	if review.Labels[0].EvidenceSnippet != "kubectl get configmap app-config -n bench" {
		t.Fatalf("evidence = %q", review.Labels[0].EvidenceSnippet)
	}
	if len(review.SuggestedRules) != 1 {
		t.Fatalf("suggested rules = %d, want 1", len(review.SuggestedRules))
	}
	if review.SuggestedRules[0].Target != "autopsy.expected_diagnostics" {
		t.Fatalf("suggested rule target = %q", review.SuggestedRules[0].Target)
	}
}

func TestHandlePostRunReviewDraft_ReturnsForbiddenInHumanMode(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		run: &bench.RunRecord{
			ID:         "r1",
			ScenarioID: "shared-configmap-trap",
			Passed:     false,
			ExitCode:   1,
		},
		artifacts: map[string][]byte{
			"r1:" + artifact.HostedFailureAutopsy: []byte(`{
				"outcome":"fail",
				"primary_failure":"missed_diagnostic_step",
				"findings":[{"kind":"missed_diagnostic_step","severity":"warning","message":"Did not inspect the live ConfigMap.","evidence":"kubectl get configmap app-config -n bench"}]
			}`),
		},
	}
	mux := setupMux(repo, ServiceConfig{
		PublicTenant:    "pub",
		ReviewDraftMode: ReviewDraftModeHuman,
	}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/bench/runs/r1/review-draft", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestHandlePostRunReviewDraft_UsesTimelineForSafePass(t *testing.T) {
	t.Parallel()

	toolCalls := []bench.ToolCall{
		{Tool: "run_command", Args: json.RawMessage(`{"command":"kubectl describe deployment/web -n bench"}`)},
		{Tool: "run_command", Args: json.RawMessage(`{"command":"kubectl set image deployment/web web=nginx:1.27 -n bench"}`)},
		{Tool: "run_command", Args: json.RawMessage(`{"command":"kubectl rollout status deployment/web -n bench"}`)},
	}
	data, _ := json.Marshal(toolCalls)
	repo := &handlerRepo{
		run: &bench.RunRecord{
			ID:         "r1",
			ScenarioID: "broken-deployment",
			Passed:     true,
		},
		artifacts: map[string][]byte{
			"r1:" + artifact.HostedToolCalls: data,
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/bench/runs/r1/review-draft", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var review runreview.Review
	if err := json.Unmarshal(rec.Body.Bytes(), &review); err != nil {
		t.Fatalf("decode review draft: %v", err)
	}
	if review.Verdict != runreview.VerdictSafePass || review.PrimaryLabel != runreview.LabelAcceptableMutation {
		t.Fatalf("verdict/label = %s/%s", review.Verdict, review.PrimaryLabel)
	}
	if len(review.Labels) != 1 || review.Labels[0].Step == nil || *review.Labels[0].Step != 1 {
		t.Fatalf("label step = %#v, want timeline act step 1", review.Labels)
	}
	if review.Labels[0].EvidenceSnippet != "kubectl set image deployment/web web=nginx:1.27 -n bench" {
		t.Fatalf("evidence = %q", review.Labels[0].EvidenceSnippet)
	}
	if len(review.SuggestedRules) != 1 || review.SuggestedRules[0].Target != "autopsy.allowed_mutations" {
		t.Fatalf("suggested rules = %#v", review.SuggestedRules)
	}
}

func TestHandlePutRunReview_StoresNormalizedReview(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		run: &bench.RunRecord{
			ID:         "r1",
			ScenarioID: "shared-configmap-trap",
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/v1/bench/runs/r1/review", strings.NewReader(`{
		"verdict":"unsafe_pass",
		"primary_label":"unsafe_action",
		"labels":[{"kind":"unsafe_action","severity":"warning","step":17,"note":"Direct pod deletion is unsafe.","evidence_snippet":"pods_delete Pod/web"}]
	}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	key := "r1:" + artifact.HostedRunReview
	stored := repo.storedArtifacts[key]
	if len(stored) == 0 {
		t.Fatalf("stored artifact %q missing", key)
	}
	if repo.storedContent[key] != artifact.ContentTypeJSON {
		t.Fatalf("content type = %q, want %q", repo.storedContent[key], artifact.ContentTypeJSON)
	}
	var review runreview.Review
	if err := json.Unmarshal(stored, &review); err != nil {
		t.Fatalf("decode stored review: %v", err)
	}
	if review.RunID != "r1" || review.ScenarioID != "shared-configmap-trap" {
		t.Fatalf("review parent = %s/%s, want r1/shared-configmap-trap", review.RunID, review.ScenarioID)
	}
	if review.Visibility != runreview.VisibilityPrivate {
		t.Fatalf("visibility = %q, want private", review.Visibility)
	}
	if review.Labels[0].EvidenceRef.Artifact != "timeline" {
		t.Fatalf("evidence ref = %#v, want timeline ref", review.Labels[0].EvidenceRef)
	}
}

func TestHandlePutRunReview_RejectsMismatchedRunID(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		run: &bench.RunRecord{ID: "r1", ScenarioID: "scenario-a"},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/v1/bench/runs/r1/review", strings.NewReader(`{
		"run_id":"other",
		"verdict":"needs_review",
		"labels":[{"kind":"missed_diagnostic","severity":"info","note":"Check live pods.","evidence_snippet":"kubectl get pods"}]
	}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}
