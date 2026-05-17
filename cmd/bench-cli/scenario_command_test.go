package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScenarioListCommand(t *testing.T) {
	dir := t.TempDir()
	scenarioDir := filepath.Join(dir, "kubernetes", "broken-deployment")
	if err := os.MkdirAll(scenarioDir, 0755); err != nil {
		t.Fatal(err)
	}
	yamlContent := `id: broken-deployment
title: Fix broken deployment
category: kubernetes
prompt: prompts/task.md
break:
  type: kubectl
  command: "patch deployment web -n bench"
checks:
  - type: deployment-ready
    namespace: bench
    name: web
`
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"scenario", "list", "--scenarios-dir", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !strings.Contains(buf.String(), "kubernetes/broken-deployment") {
		t.Fatalf("expected relative scenario path in output, got %q", buf.String())
	}
}

func TestPushScenariosIncludesAutopsyDescription(t *testing.T) {
	dir := t.TempDir()
	scenarioDir := filepath.Join(dir, "kubernetes", "network-policy-fix")
	if err := os.MkdirAll(filepath.Join(scenarioDir, "prompts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "prompts", "task.md"), []byte("Fix segmentation."), 0644); err != nil {
		t.Fatal(err)
	}
	yamlContent := `id: network-policy-fix
title: Fix network policy
description: Fix tier segmentation.
category: kubernetes
prompt: prompts/task.md
break:
  type: kubectl
  command: "apply"
checks:
  - type: deployment-ready
    namespace: bench
    name: frontend
autopsy:
  description: |
    Root cause: NetworkPolicy selects too many pods.
    Safe repair: narrow the selector.
`
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/bench/scenarios/sync" {
			t.Fatalf("path = %q, want /v1/bench/scenarios/sync", r.URL.Path)
		}
		if gotAuth := r.Header.Get("Authorization"); gotAuth != "Bearer test-key" {
			t.Fatalf("Authorization = %q", gotAuth)
		}
		var body struct {
			Scenarios []struct {
				AutopsyDescription string `json:"autopsy_description"`
			} `json:"scenarios"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(body.Scenarios) != 1 {
			t.Fatalf("scenarios = %d, want 1", len(body.Scenarios))
		}
		got = body.Scenarios[0].AutopsyDescription
		_, _ = w.Write([]byte(`{"ok":true,"total":1,"upserted":1}`))
	}))
	defer server.Close()

	if err := pushScenarios(dir, server.URL, "test-key"); err != nil {
		t.Fatalf("pushScenarios: %v", err)
	}
	if !strings.Contains(got, "Root cause: NetworkPolicy") {
		t.Fatalf("autopsy_description = %q", got)
	}
}
