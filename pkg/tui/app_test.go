package tui

import (
	"os"
	"path/filepath"
	"testing"

	"samebits.com/evidra-infra-bench/pkg/harness"
)

func TestAgentCommandRequired(t *testing.T) {
	t.Parallel()

	if !agentCommandRequired(LabConfig{}) {
		t.Fatal("expected empty config to require agent command")
	}
	if agentCommandRequired(LabConfig{Provider: "bifrost"}) {
		t.Fatal("provider mode should not require agent command")
	}
	if agentCommandRequired(LabConfig{AgentCommand: "/usr/bin/agent"}) {
		t.Fatal("explicit agent command should satisfy requirement")
	}
}

func TestNewApp_UsesConfiguredRunsDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	scenariosDir := filepath.Join(root, "scenarios")
	scenarioDir := filepath.Join(scenariosDir, "kubernetes", "broken-deployment")
	if err := os.MkdirAll(filepath.Join(scenarioDir, "prompts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "prompts", "task.md"), []byte("fix it"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(`id: broken-deployment
title: Fix broken deployment
category: kubernetes
prompt: prompts/task.md
checks:
  - type: deployment-ready
    namespace: bench
    name: web
`), 0644); err != nil {
		t.Fatal(err)
	}

	runsDir := filepath.Join(root, "custom-runs")
	app, err := NewApp(scenariosDir, filepath.Join(root, ".lab-config.yaml"), LabConfig{
		RunsDir: runsDir,
	}, harness.Deps{})
	if err != nil {
		t.Fatalf("NewApp failed: %v", err)
	}
	if app.runsDir != runsDir {
		t.Fatalf("runsDir = %q, want %q", app.runsDir, runsDir)
	}
}
