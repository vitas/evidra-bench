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
		ScenarioID      string `json:"scenario_id"`
		Model           string `json:"model"`
		Provider        string `json:"provider"`
		TriggerID       string `json:"trigger_id"`
		TriggerURL      string `json:"trigger_url"`
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
