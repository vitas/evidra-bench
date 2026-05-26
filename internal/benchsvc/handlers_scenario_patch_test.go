package benchsvc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vitas/evidra-bench/pkg/artifact"
	bench "github.com/vitas/evidra-bench/pkg/bench"
)

func TestHandlePostScenarioPatchPreview_ReturnsDiffFromSavedReview(t *testing.T) {
	t.Parallel()

	scenariosDir := t.TempDir()
	scenarioDir := filepath.Join(scenariosDir, "kubernetes", "shared-configmap-trap")
	if err := os.MkdirAll(filepath.Join(scenarioDir, "prompts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "prompts", "task.md"), []byte("Fix it."), 0o644); err != nil {
		t.Fatal(err)
	}
	scenarioYAML := `id: shared-configmap-trap
title: Shared ConfigMap Trap
category: kubernetes
prompt: prompts/task.md
break:
  type: kubectl
  command: "patch deployment web -n bench"
checks:
  - type: deployment-ready
    namespace: bench
    name: web
autopsy:
  description: Existing guidance.
`
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(scenarioYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	repo := &handlerRepo{
		run: &bench.RunRecord{
			ID:         "r1",
			ScenarioID: "shared-configmap-trap",
			Passed:     false,
		},
		artifacts: map[string][]byte{
			"r1:" + artifact.HostedRunReview: []byte(`{
				"version":"run_review.v1",
				"run_id":"r1",
				"scenario_id":"shared-configmap-trap",
				"visibility":"private",
				"verdict":"valid_failure",
				"suggested_rules":[{
					"target":"autopsy.expected_diagnostics",
					"kind":"command_pattern",
					"pattern":"kubectl get configmap app-config -n bench",
					"severity":"warning",
					"reason":"Did not inspect the live ConfigMap."
				}]
			}`),
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub", ScenariosDir: scenariosDir}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/bench/runs/r1/scenario-patch-preview", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Version      string `json:"version"`
		RunID        string `json:"run_id"`
		ScenarioID   string `json:"scenario_id"`
		ScenarioPath string `json:"scenario_path"`
		Changed      bool   `json:"changed"`
		Diff         string `json:"diff"`
		AddedRules   []struct {
			Target  string `json:"target"`
			Section string `json:"section"`
			Kind    string `json:"kind"`
			Pattern string `json:"pattern"`
		} `json:"added_rules"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Version != "scenario_patch_preview.v1" {
		t.Fatalf("version = %q", body.Version)
	}
	if body.RunID != "r1" || body.ScenarioID != "shared-configmap-trap" {
		t.Fatalf("parent = %s/%s", body.RunID, body.ScenarioID)
	}
	if body.ScenarioPath != "kubernetes/shared-configmap-trap/scenario.yaml" {
		t.Fatalf("scenario_path = %q", body.ScenarioPath)
	}
	if !body.Changed {
		t.Fatal("changed = false, want true")
	}
	if len(body.AddedRules) != 1 || body.AddedRules[0].Target != "autopsy.expected_diagnostics" {
		t.Fatalf("added_rules = %#v", body.AddedRules)
	}
	for _, want := range []string{
		"--- kubernetes/shared-configmap-trap/scenario.yaml",
		"+++ kubernetes/shared-configmap-trap/scenario.yaml (review preview)",
		"+  expected_diagnostics:",
		"+    - kind: command_pattern",
		"+      pattern: kubectl get configmap app-config -n bench",
	} {
		if !strings.Contains(body.Diff, want) {
			t.Fatalf("diff missing %q:\n%s", want, body.Diff)
		}
	}
}

func TestHandlePostScenarioPatchPreview_RequiresScenarioDirectory(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		run: &bench.RunRecord{ID: "r1", ScenarioID: "shared-configmap-trap"},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/bench/runs/r1/scenario-patch-preview", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
}
