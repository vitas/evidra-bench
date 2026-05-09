package benchsvc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
		{ID: "mcp-1", ScenarioID: "s1", Model: "sonnet", EvidenceMode: "mcp", Passed: true, Duration: 30, EstimatedCost: 3.0},
		{ID: "mcp-2", ScenarioID: "s2", Model: "sonnet", EvidenceMode: "mcp", Passed: false, Duration: 40, EstimatedCost: 4.0},
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

func TestHandleLeaderboard_ScenarioFilter(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		runs: []bench.RunRecord{
			{ID: "r1", ScenarioID: "s1", Model: "sonnet", Passed: true, Duration: 10},
			{ID: "r2", ScenarioID: "s2", Model: "sonnet", Passed: false, Duration: 10},
			{ID: "r3", ScenarioID: "s3", Model: "sonnet", Passed: true, Duration: 10},
			{ID: "r4", ScenarioID: "s1", Model: "opus", Passed: false, Duration: 10},
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "t1")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/leaderboard?scenarios=s1,s3", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got, want := repo.lastScenarios, []string{"s1", "s3"}; !equalStringSlices(got, want) {
		t.Fatalf("scenarios = %v, want %v", got, want)
	}

	var body struct {
		Models []bench.LeaderboardEntry `json:"models"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body.Models) != 2 {
		t.Fatalf("len(models) = %d, want 2", len(body.Models))
	}
	if body.Models[0].Model != "sonnet" || body.Models[0].Runs != 2 || body.Models[0].PassRate != 100 {
		t.Fatalf("first model = %+v, want sonnet with 2 passing filtered runs", body.Models[0])
	}
	if body.Models[1].Model != "opus" || body.Models[1].Runs != 1 || body.Models[1].PassRate != 0 {
		t.Fatalf("second model = %+v, want opus with one failing filtered run", body.Models[1])
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
