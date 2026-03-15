package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateHTML_IncludesChaosMetadata(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	scenariosDir := filepath.Join(root, "scenarios")
	runsDir := filepath.Join(root, "runs")
	outputPath := filepath.Join(root, "report.html")

	scenarioDir := filepath.Join(scenariosDir, "kubernetes", "pod-kill-during-repair")
	if err := os.MkdirAll(filepath.Join(scenarioDir, "prompts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "prompts", "task.md"), []byte("Fix it."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(`id: pod-kill-during-repair
title: Pod kill during repair
category: kubernetes
prompt: prompts/task.md
chaos:
  steps:
    - at: 10s
      name: kill-web
      type: kubectl
      args: [delete, pod, web-0]
checks:
  - type: deployment-ready
    namespace: bench
    name: web
`), 0644); err != nil {
		t.Fatal(err)
	}

	runDir := filepath.Join(runsDir, "20260315-120000-pod-kill-during-repair-cli")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "run.json"), []byte(`{
  "scenario_id": "pod-kill-during-repair",
  "adapter": "cli",
  "passed": true,
  "start_time": "2026-03-15T12:00:00Z",
  "end_time": "2026-03-15T12:00:30Z",
  "exit_code": 0,
  "chaos_enabled": true,
  "chaos_mode": "once",
  "chaos_step_count": 1,
  "metadata": {
    "provider": "claude",
    "model": "sonnet"
  }
}`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := GenerateHTML(scenariosDir, runsDir, outputPath); err != nil {
		t.Fatalf("GenerateHTML failed: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)
	for _, want := range []string{
		"Chaos",
		"pod-kill-during-repair",
		"once / 1",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("report missing %q\n%s", want, html)
		}
	}
}
