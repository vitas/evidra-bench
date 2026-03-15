package artifact

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriter_CreatesRunBundle(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := NewWriter(dir)
	out, err := w.Write(RunBundle{
		ScenarioID: "broken-deployment",
		Adapter:    "cli",
		StartTime:  time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC),
		EndTime:    time.Date(2026, 3, 14, 10, 3, 0, 0, time.UTC),
		ExitCode:   0,
		Passed:     true,
	})
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out.Path, "run.json")); err != nil {
		t.Fatalf("missing run.json: %v", err)
	}
}

func TestWriter_CreatesEvidraDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := NewWriter(dir)
	out, err := w.Write(RunBundle{
		ScenarioID: "test",
		Adapter:    "cli",
		StartTime:  time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(out.Path, "evidra"))
	if err != nil {
		t.Fatalf("missing evidra dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("evidra is not a directory")
	}
}

func TestWriter_WritesTextArtifacts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := NewWriter(dir)
	out, err := w.Write(RunBundle{
		ScenarioID: "test",
		Adapter:    "cli",
		StartTime:  time.Now(),
		Prompt:     "fix the deployment",
		Transcript: "agent output here",
		Stdout:     "stdout content",
		Stderr:     "stderr content",
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"prompt.txt", "transcript.txt", "stdout.txt", "stderr.txt"} {
		if _, err := os.Stat(filepath.Join(out.Path, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
}

func TestWriter_SkipsEmptyTextFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := NewWriter(dir)
	out, err := w.Write(RunBundle{
		ScenarioID: "test",
		Adapter:    "cli",
		StartTime:  time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	// No prompt, transcript, stdout, stderr — files should not exist.
	for _, name := range []string{"prompt.txt", "transcript.txt", "stdout.txt", "stderr.txt"} {
		if _, err := os.Stat(filepath.Join(out.Path, name)); err == nil {
			t.Fatalf("expected %s to not exist", name)
		}
	}
}

func TestWriter_WritesToolCalls(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := NewWriter(dir)
	tc, _ := json.Marshal([]map[string]string{{"tool": "kubectl"}})
	out, err := w.Write(RunBundle{
		ScenarioID: "test",
		Adapter:    "cli",
		StartTime:  time.Now(),
		ToolCalls:  tc,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out.Path, "tool-calls.json")); err != nil {
		t.Fatalf("missing tool-calls.json: %v", err)
	}
}

func TestWriter_RunJSONContainsScenarioID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := NewWriter(dir)
	out, err := w.Write(RunBundle{
		ScenarioID: "my-scenario",
		Adapter:    "mcp",
		StartTime:  time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(out.Path, "run.json"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed RunBundle
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.ScenarioID != "my-scenario" {
		t.Fatalf("unexpected scenario_id: %s", parsed.ScenarioID)
	}
}

func TestWriter_WritesChaosArtifacts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	w := NewWriter(dir)
	chaosTimeline, _ := json.Marshal(map[string]any{
		"mode": "once",
		"events": []map[string]any{
			{"name": "kill-web", "type": "kubectl", "success": true},
		},
	})

	out, err := w.Write(RunBundle{
		ScenarioID:      "chaos-scenario",
		Adapter:         "cli",
		StartTime:       time.Now(),
		ChaosEnabled:    true,
		ChaosMode:       "once",
		ChaosStepCount:  1,
		ChaosTimeline:   chaosTimeline,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(out.Path, "chaos.json")); err != nil {
		t.Fatalf("missing chaos.json: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(out.Path, "run.json"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed RunBundle
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if !parsed.ChaosEnabled {
		t.Fatal("run.json chaos_enabled = false, want true")
	}
	if parsed.ChaosMode != "once" {
		t.Fatalf("run.json chaos_mode = %q, want once", parsed.ChaosMode)
	}
	if parsed.ChaosStepCount != 1 {
		t.Fatalf("run.json chaos_step_count = %d, want 1", parsed.ChaosStepCount)
	}
}
