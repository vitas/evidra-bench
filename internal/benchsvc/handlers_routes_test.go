package benchsvc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	bench "samebits.com/evidra-infra-bench/pkg/bench"
)

// ---------- Leaderboard ----------

func TestHandleLeaderboard_ReturnsModels(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		leaders: []bench.LeaderboardEntry{
			{Model: "sonnet", Runs: 10, PassRate: 0.9},
			{Model: "opus", Runs: 5, PassRate: 1.0},
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "t1")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/leaderboard", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if _, ok := body["models"]; !ok {
		t.Fatal("response missing 'models' key")
	}
	var models []bench.LeaderboardEntry
	if err := json.Unmarshal(body["models"], &models); err != nil {
		t.Fatalf("decode models: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("len(models) = %d, want 2", len(models))
	}
}

func TestHandleLeaderboard_DefaultsToProxy(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "t1")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/leaderboard", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if repo.lastMode != "" {
		t.Fatalf("evidence_mode = %q, want empty (all)", repo.lastMode)
	}

	var body struct {
		EvidenceMode string `json:"evidence_mode"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.EvidenceMode != "" {
		t.Fatalf("response evidence_mode = %q, want empty", body.EvidenceMode)
	}
}

func TestHandleLeaderboard_EvidenceModeFiltersAndAggregates(t *testing.T) {
	t.Parallel()

	sharedRuns := []bench.RunRecord{
		{ID: "baseline-1", ScenarioID: "s1", Model: "sonnet", EvidenceMode: "none", Passed: true, Duration: 10, EstimatedCost: 1.0},
		{ID: "baseline-2", ScenarioID: "s2", Model: "sonnet", EvidenceMode: "none", Passed: false, Duration: 20, EstimatedCost: 2.0},
		{ID: "evidra-1", ScenarioID: "s1", Model: "sonnet", EvidenceMode: "mcp", Passed: true, Duration: 30, EstimatedCost: 3.0},
		{ID: "evidra-2", ScenarioID: "s2", Model: "sonnet", EvidenceMode: "mcp", Passed: false, Duration: 40, EstimatedCost: 4.0},
	}

	tests := []struct {
		name         string
		mode         string
		wantRuns     int
		wantPassRate float64
	}{
		{name: "baseline only", mode: "none", wantRuns: 2, wantPassRate: 50.0},
		{name: "mcp", mode: "mcp", wantRuns: 2, wantPassRate: 50.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := &handlerRepo{runs: sharedRuns}
			mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "t1")

			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/bench/leaderboard?evidence_mode="+tt.mode, nil)
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}

			var body struct {
				Models       []bench.LeaderboardEntry `json:"models"`
				EvidenceMode string                   `json:"evidence_mode"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body.EvidenceMode != tt.mode {
				t.Fatalf("response evidence_mode = %q, want %q", body.EvidenceMode, tt.mode)
			}
			if len(body.Models) != 1 {
				t.Fatalf("len(models) = %d, want 1", len(body.Models))
			}
			if body.Models[0].Model != "sonnet" {
				t.Fatalf("model = %q, want sonnet", body.Models[0].Model)
			}
			if body.Models[0].Runs != tt.wantRuns {
				t.Fatalf("runs = %d, want %d", body.Models[0].Runs, tt.wantRuns)
			}
			if body.Models[0].PassRate != tt.wantPassRate {
				t.Fatalf("pass_rate = %v, want %v", body.Models[0].PassRate, tt.wantPassRate)
			}
		})
	}
}

func TestHandleLeaderboard_503WhenNoPublicTenant(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: ""}, "t1")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/leaderboard", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

// ---------- Models ----------

func TestHandleListModels_ReturnsModels(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		enabledModels: []EnabledModel{
			{
				ID:                "gemini-2.5-flash",
				DisplayName:       "Gemini 2.5 Flash",
				Provider:          "google",
				InputCostPerMtok:  0.15,
				OutputCostPerMtok: 0.60,
			},
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/models", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body struct {
		Models []EnabledModel `json:"models"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(body.Models))
	}
	if body.Models[0].ID != "gemini-2.5-flash" {
		t.Fatalf("id = %q, want gemini-2.5-flash", body.Models[0].ID)
	}
	if repo.lastTenant != "tenant-a" {
		t.Fatalf("tenant = %q, want tenant-a", repo.lastTenant)
	}
}

func TestHandleUpsertTenantProvider_Returns404WhileDisabled(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/v1/bench/models/gemini-2.5-flash/provider", strings.NewReader(`{"api_key":"sk-secret","rate_limit":10}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	if repo.lastTenant != "" {
		t.Fatalf("tenant = %q, want empty because route is disabled", repo.lastTenant)
	}
	if repo.lastModelID != "" {
		t.Fatalf("modelID = %q, want empty because route is disabled", repo.lastModelID)
	}
}

func TestHandleDeleteTenantProvider_Returns404WhileDisabled(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/v1/bench/models/gemini-2.5-flash/provider", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	if repo.lastTenant != "" {
		t.Fatalf("tenant = %q, want empty because route is disabled", repo.lastTenant)
	}
	if repo.lastModelID != "" {
		t.Fatalf("modelID = %q, want empty because route is disabled", repo.lastModelID)
	}
}

func TestHandleUpdateGlobalModel_Returns204(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	svc := NewService(repo, ServiceConfig{})
	handler := HandleUpdateGlobalModel(svc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/v1/admin/bench/models/gemini-2.5-flash", strings.NewReader(`{"api_key_env":"CUSTOM_KEY","api_base_url":"https://gw.example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("model_id", "gemini-2.5-flash")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if repo.lastModelID != "gemini-2.5-flash" {
		t.Fatalf("modelID = %q, want gemini-2.5-flash", repo.lastModelID)
	}
	if repo.lastGlobalCfg.APIKeyEnv != "CUSTOM_KEY" {
		t.Fatalf("api_key_env = %q, want CUSTOM_KEY", repo.lastGlobalCfg.APIKeyEnv)
	}
	if repo.lastGlobalCfg.APIBaseURL != "https://gw.example.com" {
		t.Fatalf("api_base_url = %q, want https://gw.example.com", repo.lastGlobalCfg.APIBaseURL)
	}
}

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
	req := httptest.NewRequest("GET", "/v1/bench/runs?model=sonnet&scenario=broken-deployment&evidence_mode=mcp&limit=10&offset=5", nil)
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
	if f.EvidenceMode != "mcp" {
		t.Errorf("EvidenceMode = %q, want mcp", f.EvidenceMode)
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
		{ID: "evidra-1", ScenarioID: "s3", Model: "sonnet", EvidenceMode: "mcp"},
		{ID: "evidra-2", ScenarioID: "s4", Model: "sonnet", EvidenceMode: "mcp"},
	}

	tests := []struct {
		name    string
		mode    string
		wantIDs []string
	}{
		{name: "baseline only", mode: "none", wantIDs: []string{"baseline-1", "baseline-2"}},
		{name: "mcp", mode: "mcp", wantIDs: []string{"evidra-1", "evidra-2"}},
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

// ---------- Ingest ----------

func TestHandleIngestRun_ValidPayload(t *testing.T) {
	t.Parallel()

	// IngestRun calls BeginTx which our handlerRepo doesn't support,
	// so we use a dedicated repo that returns a fakeTx.
	repo := &ingestRepo{}
	svc := NewService(repo, ServiceConfig{PublicTenant: "pub"})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("tenant-a"))

	payload := `{"id":"r1","scenario_id":"s1","model":"sonnet"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/bench/runs", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["ok"] != true {
		t.Fatalf("ok = %v, want true", body["ok"])
	}
}

func TestHandleIngestRun_RejectsMissingFields(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	tests := []struct {
		name    string
		payload string
	}{
		{"missing id", `{"scenario_id":"s1","model":"m1"}`},
		{"missing scenario_id", `{"id":"r1","model":"m1"}`},
		{"missing model", `{"id":"r1","scenario_id":"s1"}`},
		{"empty body", `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/bench/runs", strings.NewReader(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
		})
	}
}

func TestHandleIngestBatch_ImportsRuns(t *testing.T) {
	t.Parallel()

	repo := &ingestRepo{batchCount: 3}
	svc := NewService(repo, ServiceConfig{PublicTenant: "pub"})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("tenant-a"))

	payload := `{"runs":[
		{"id":"r1","scenario_id":"s1","model":"m1"},
		{"id":"r2","scenario_id":"s1","model":"m1"},
		{"id":"r3","scenario_id":"s2","model":"m2"}
	]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/bench/runs/batch", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["ok"] != true {
		t.Fatalf("ok = %v, want true", body["ok"])
	}
	if int(body["imported"].(float64)) != 3 {
		t.Fatalf("imported = %v, want 3", body["imported"])
	}
}

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
		artifact: data,
		artCT:    "application/json",
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
		{ID: "evidra-1", ScenarioID: "s3", Model: "sonnet", EvidenceMode: "mcp", Passed: true},
		{ID: "evidra-2", ScenarioID: "s4", Model: "sonnet", EvidenceMode: "mcp", Passed: false},
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
			Models:    []string{"sonnet", "opus"},
			Providers: []string{"anthropic", "bifrost"},
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

// ---------- Ingest support: fake that supports transactions ----------

// ingestRepo wraps handlerRepo with a fake transaction that accepts Exec and Commit.
type ingestRepo struct {
	handlerRepo
	batchCount int
}

func (r *ingestRepo) UpsertScenarios(_ context.Context, _ []bench.ScenarioSummary) (int, error) {
	return 0, nil
}
func (r *ingestRepo) BeginTx(_ context.Context) (pgx.Tx, error) {
	// Reuse fakeTx from service_batch_test.go (same package).
	// Supply enough "INSERT 0 1" tags so IngestRunBatch counts rows as inserted.
	tags := make([]pgconn.CommandTag, 20)
	for i := range tags {
		tags[i] = pgconn.NewCommandTag("INSERT 0 1")
	}
	return &fakeTx{execTags: tags}, nil
}

func (r *ingestRepo) InsertRunBatch(_ context.Context, _ string, _ []bench.RunRecord) (int, error) {
	return r.batchCount, nil
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
