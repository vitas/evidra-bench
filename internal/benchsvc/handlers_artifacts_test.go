package benchsvc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	bench "github.com/vitas/evidra-bench/pkg/bench"
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
