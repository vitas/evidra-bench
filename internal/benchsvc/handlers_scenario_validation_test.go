package benchsvc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vitas/evidra-bench/pkg/artifact"
	bench "github.com/vitas/evidra-bench/pkg/bench"
)

func TestHandlePostScenarioPatchValidation_QueuesRerunFromSourceRun(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	startedCh := make(chan struct{})
	repo := &handlerRepo{
		run: &bench.RunRecord{
			ID:                "source-run",
			ScenarioID:        "shared-configmap-trap",
			Model:             "sonnet",
			Provider:          "anthropic",
			Adapter:           "provider",
			ToolServer:        "kubernetes-mcp",
			ToolServerVersion: "1.2.3",
			SkillID:           "k8s-admin",
			SkillVersion:      "2026-05-13",
			SkillSource:       "local-temp",
			SkillSHA256:       "abc123",
			CreatedAt:         time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC),
		},
		modelProvider: &ModelProviderInfo{Provider: "anthropic"},
		artifacts: map[string][]byte{
			"source-run:" + artifact.HostedScenarioPatchPreview: []byte(`{
				"version":"scenario_patch_preview.v1",
				"run_id":"source-run",
				"scenario_id":"shared-configmap-trap",
				"changed":true,
				"diff":"--- scenario.yaml\n+++ scenario.yaml (review preview)\n",
				"artifact_url":"/v1/bench/runs/source-run/scenario-patch-preview",
				"diff_url":"/v1/bench/runs/source-run/scenario-patch.diff"
			}`),
		},
	}
	spy := &spyExecutor{startedCh: startedCh}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Executor:     spy,
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("tenant-a"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/bench/runs/source-run/scenario-patch-validation", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	select {
	case <-startedCh:
	case <-time.After(time.Second):
		t.Fatal("executor Start was not called")
	}
	var body struct {
		Version         string `json:"version"`
		SourceRunID     string `json:"source_run_id"`
		SourceRunURL    string `json:"source_run_url"`
		ScenarioID      string `json:"scenario_id"`
		Model           string `json:"model"`
		Provider        string `json:"provider"`
		TriggerID       string `json:"trigger_id"`
		TriggerURL      string `json:"trigger_url"`
		ValidationURL   string `json:"validation_url"`
		Status          string `json:"status"`
		PatchPreviewURL string `json:"patch_preview_url"`
		PatchDiffURL    string `json:"patch_diff_url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Version != "scenario_patch_validation.v1" {
		t.Fatalf("version = %q", body.Version)
	}
	if body.SourceRunID != "source-run" || body.ScenarioID != "shared-configmap-trap" {
		t.Fatalf("source/scenario = %s/%s", body.SourceRunID, body.ScenarioID)
	}
	if body.SourceRunURL != "/v1/bench/runs/source-run" || body.ValidationURL != "/v1/bench/runs/source-run/scenario-patch-validation" {
		t.Fatalf("run urls = %#v", body)
	}
	if body.Model != "sonnet" || body.Provider != "anthropic" {
		t.Fatalf("model/provider = %s/%s", body.Model, body.Provider)
	}
	if body.TriggerID == "" || body.TriggerURL != "/v1/bench/trigger/"+body.TriggerID || body.Status != "pending" {
		t.Fatalf("trigger fields = %#v", body)
	}
	if body.PatchPreviewURL != "/v1/bench/runs/source-run/scenario-patch-preview" ||
		body.PatchDiffURL != "/v1/bench/runs/source-run/scenario-patch.diff" {
		t.Fatalf("patch urls = %#v", body)
	}
	stored := store.Get(body.TriggerID)
	if stored == nil {
		t.Fatal("stored trigger job missing")
		return
	}
	if stored.Model != "sonnet" || stored.Provider != "anthropic" || stored.Total != 1 {
		t.Fatalf("stored trigger = %#v", stored)
	}
	if len(stored.Progress) != 1 || stored.Progress[0].Scenario != "shared-configmap-trap" {
		t.Fatalf("stored progress = %#v", stored.Progress)
	}
	if stored.ToolServer != "kubernetes-mcp" || stored.ToolServerVersion != "1.2.3" {
		t.Fatalf("stored tool server = %s/%s", stored.ToolServer, stored.ToolServerVersion)
	}
	if stored.SkillID != "k8s-admin" || stored.SkillVersion != "2026-05-13" || stored.SkillSHA256 != "abc123" {
		t.Fatalf("stored skill = %s/%s/%s", stored.SkillID, stored.SkillVersion, stored.SkillSHA256)
	}

	storedKey := "source-run:" + artifact.HostedScenarioPatchValidation
	storedArtifact := repo.storedArtifacts[storedKey]
	if len(storedArtifact) == 0 {
		t.Fatalf("stored validation artifact %s missing", storedKey)
	}
	if repo.storedContent[storedKey] != artifact.ContentTypeJSON {
		t.Fatalf("stored validation content type = %q", repo.storedContent[storedKey])
	}
	var storedValidation struct {
		Version       string `json:"version"`
		SourceRunID   string `json:"source_run_id"`
		SourceRunURL  string `json:"source_run_url"`
		TriggerID     string `json:"trigger_id"`
		ValidationURL string `json:"validation_url"`
	}
	if err := json.Unmarshal(storedArtifact, &storedValidation); err != nil {
		t.Fatalf("decode stored validation: %v", err)
	}
	if storedValidation.Version != "scenario_patch_validation.v1" ||
		storedValidation.SourceRunID != "source-run" ||
		storedValidation.SourceRunURL != "/v1/bench/runs/source-run" ||
		storedValidation.TriggerID != body.TriggerID ||
		storedValidation.ValidationURL != "/v1/bench/runs/source-run/scenario-patch-validation" {
		t.Fatalf("stored validation = %#v", storedValidation)
	}
}

func TestHandleGetScenarioPatchValidation_ReturnsStoredArtifactWithTriggerProgress(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	store.Create(&TriggerJob{
		ID:        "job-1",
		Status:    "completed",
		Model:     "sonnet",
		Provider:  "anthropic",
		Total:     1,
		Completed: 1,
		Passed:    1,
		Failed:    0,
		RunIDs:    []string{"validation-run"},
		Progress: []ScenarioProgress{{
			Scenario: "shared-configmap-trap",
			Status:   "passed",
			RunID:    "validation-run",
		}},
	})
	repo := &handlerRepo{
		artifacts: map[string][]byte{
			"source-run:" + artifact.HostedRunReview: []byte(`{
				"version":"run_review.v1",
				"visibility":"public",
				"verdict":"unsafe_pass"
			}`),
			"source-run:" + artifact.HostedScenarioPatchValidation: []byte(`{
				"version":"scenario_patch_validation.v1",
				"source_run_id":"source-run",
				"source_run_url":"/v1/bench/runs/source-run",
				"scenario_id":"shared-configmap-trap",
				"model":"sonnet",
				"provider":"anthropic",
				"trigger_id":"job-1",
				"trigger_url":"/v1/bench/trigger/job-1",
				"validation_url":"/v1/bench/runs/source-run/scenario-patch-validation",
				"status":"pending",
				"patch_preview_url":"/v1/bench/runs/source-run/scenario-patch-preview",
				"patch_diff_url":"/v1/bench/runs/source-run/scenario-patch.diff"
			}`),
		},
	}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("tenant-a"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/runs/source-run/scenario-patch-validation", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Status           string   `json:"status"`
		Total            int      `json:"total"`
		Completed        int      `json:"completed"`
		Passed           int      `json:"passed"`
		Failed           int      `json:"failed"`
		ValidationRunIDs []string `json:"validation_run_ids"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "completed" || body.Total != 1 || body.Completed != 1 || body.Passed != 1 || body.Failed != 0 {
		t.Fatalf("validation progress = %#v", body)
	}
	if len(body.ValidationRunIDs) != 1 || body.ValidationRunIDs[0] != "validation-run" {
		t.Fatalf("validation run ids = %#v", body.ValidationRunIDs)
	}
}

func TestHandlePostScenarioPatchValidation_RequiresChangedPatchPreview(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{
		run:           &bench.RunRecord{ID: "source-run", ScenarioID: "shared-configmap-trap", Model: "sonnet", Provider: "anthropic"},
		modelProvider: &ModelProviderInfo{Provider: "anthropic"},
		artifacts: map[string][]byte{
			"source-run:" + artifact.HostedScenarioPatchPreview: []byte(`{
				"version":"scenario_patch_preview.v1",
				"run_id":"source-run",
				"scenario_id":"shared-configmap-trap",
				"changed":false,
				"diff":""
			}`),
		},
	}
	spy := &spyExecutor{}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Executor:     spy,
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("tenant-a"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/bench/runs/source-run/scenario-patch-validation", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if spy.started {
		t.Fatal("executor should not start for a no-op patch preview")
	}
}
