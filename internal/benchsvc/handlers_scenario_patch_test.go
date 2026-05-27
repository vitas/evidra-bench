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
		ArtifactURL  string `json:"artifact_url"`
		DiffURL      string `json:"diff_url"`
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
	if body.ArtifactURL != "/v1/bench/runs/r1/scenario-patch-preview" {
		t.Fatalf("artifact_url = %q", body.ArtifactURL)
	}
	if body.DiffURL != "/v1/bench/runs/r1/scenario-patch.diff" {
		t.Fatalf("diff_url = %q", body.DiffURL)
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
	storedKey := "r1:" + artifact.HostedScenarioPatchPreview
	stored := repo.storedArtifacts[storedKey]
	if len(stored) == 0 {
		t.Fatalf("stored artifact %q is empty or missing", storedKey)
	}
	if repo.storedContent[storedKey] != artifact.ContentTypeJSON {
		t.Fatalf("stored content type = %q", repo.storedContent[storedKey])
	}
	var storedBody struct {
		Version string `json:"version"`
		Diff    string `json:"diff"`
		DiffURL string `json:"diff_url"`
	}
	if err := json.Unmarshal(stored, &storedBody); err != nil {
		t.Fatalf("decode stored preview: %v", err)
	}
	if storedBody.Version != "scenario_patch_preview.v1" || storedBody.Diff != body.Diff || storedBody.DiffURL != body.DiffURL {
		t.Fatalf("stored preview = %#v, response diff_url=%q", storedBody, body.DiffURL)
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

func TestHandleGetScenarioPatchPreviewAndDiff_ReturnStoredArtifact(t *testing.T) {
	t.Parallel()

	diff := "--- kubernetes/shared-configmap-trap/scenario.yaml\n+++ kubernetes/shared-configmap-trap/scenario.yaml (review preview)\n"
	repo := &handlerRepo{
		artifacts: map[string][]byte{
			"r1:" + artifact.HostedRunReview: []byte(`{
				"version":"run_review.v1",
				"run_id":"r1",
				"scenario_id":"shared-configmap-trap",
				"visibility":"public",
				"verdict":"valid_failure"
			}`),
			"r1:" + artifact.HostedScenarioPatchPreview: []byte(`{
				"version":"scenario_patch_preview.v1",
				"run_id":"r1",
				"scenario_id":"shared-configmap-trap",
				"scenario_path":"kubernetes/shared-configmap-trap/scenario.yaml",
				"changed":true,
				"diff":"--- kubernetes/shared-configmap-trap/scenario.yaml\n+++ kubernetes/shared-configmap-trap/scenario.yaml (review preview)\n",
				"artifact_url":"/v1/bench/runs/r1/scenario-patch-preview",
				"diff_url":"/v1/bench/runs/r1/scenario-patch.diff",
				"added_rules":[],
				"skipped_rules":[]
			}`),
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	previewRec := httptest.NewRecorder()
	previewReq := httptest.NewRequest("GET", "/v1/bench/runs/r1/scenario-patch-preview", nil)
	mux.ServeHTTP(previewRec, previewReq)

	if previewRec.Code != http.StatusOK {
		t.Fatalf("preview status = %d, want %d; body: %s", previewRec.Code, http.StatusOK, previewRec.Body.String())
	}
	if contentType := previewRec.Header().Get("Content-Type"); !strings.Contains(contentType, artifact.ContentTypeJSON) {
		t.Fatalf("preview content-type = %q", contentType)
	}
	if !strings.Contains(previewRec.Body.String(), `"diff_url":"/v1/bench/runs/r1/scenario-patch.diff"`) {
		t.Fatalf("preview body missing diff_url: %s", previewRec.Body.String())
	}

	diffRec := httptest.NewRecorder()
	diffReq := httptest.NewRequest("GET", "/v1/bench/runs/r1/scenario-patch.diff", nil)
	mux.ServeHTTP(diffRec, diffReq)

	if diffRec.Code != http.StatusOK {
		t.Fatalf("diff status = %d, want %d; body: %s", diffRec.Code, http.StatusOK, diffRec.Body.String())
	}
	if contentType := diffRec.Header().Get("Content-Type"); !strings.Contains(contentType, "text/x-diff") {
		t.Fatalf("diff content-type = %q", contentType)
	}
	if diffRec.Body.String() != diff {
		t.Fatalf("diff body = %q, want %q", diffRec.Body.String(), diff)
	}
}

func TestHandleGetScenarioPatchDiff_HidesPrivateReviewFromAnonymousRead(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		artifacts: map[string][]byte{
			"r1:" + artifact.HostedRunReview: []byte(`{
				"version":"run_review.v1",
				"run_id":"r1",
				"scenario_id":"shared-configmap-trap",
				"visibility":"private",
				"verdict":"valid_failure"
			}`),
			"r1:" + artifact.HostedScenarioPatchPreview: []byte(`{
				"version":"scenario_patch_preview.v1",
				"run_id":"r1",
				"scenario_id":"shared-configmap-trap",
				"changed":true,
				"diff":"--- scenario.yaml\n+++ scenario.yaml (review preview)\n"
			}`),
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	anonymousRec := httptest.NewRecorder()
	anonymousReq := httptest.NewRequest("GET", "/v1/bench/runs/r1/scenario-patch.diff", nil)
	mux.ServeHTTP(anonymousRec, anonymousReq)

	if anonymousRec.Code != http.StatusNotFound {
		t.Fatalf("anonymous status = %d, want %d; body: %s", anonymousRec.Code, http.StatusNotFound, anonymousRec.Body.String())
	}

	authRec := httptest.NewRecorder()
	authReq := httptest.NewRequest("GET", "/v1/bench/runs/r1/scenario-patch.diff", nil)
	authReq.Header.Set("Authorization", "Bearer test-token")
	mux.ServeHTTP(authRec, authReq)

	if authRec.Code != http.StatusOK {
		t.Fatalf("authenticated status = %d, want %d; body: %s", authRec.Code, http.StatusOK, authRec.Body.String())
	}
}
