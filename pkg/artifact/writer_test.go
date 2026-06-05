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

func TestWriter_CreatesEvidenceDir(t *testing.T) {
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
	info, err := os.Stat(filepath.Join(out.Path, "evidence"))
	if err != nil {
		t.Fatalf("missing evidence dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("evidence is not a directory")
	}
}

func TestWriter_UsesRunIDForDirectoryAndRunJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	w := NewWriter(dir)
	out, err := w.Write(RunBundle{
		RunID:      "run-abc123",
		ScenarioID: "broken-deployment",
		Adapter:    "cli",
		StartTime:  time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if filepath.Base(out.Path) != "run-abc123" {
		t.Fatalf("artifact dir = %q, want run ID directory", filepath.Base(out.Path))
	}

	data, err := os.ReadFile(filepath.Join(out.Path, "run.json"))
	if err != nil {
		t.Fatalf("read run.json: %v", err)
	}
	var parsed RunBundle
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse run.json: %v", err)
	}
	if parsed.RunID != "run-abc123" {
		t.Fatalf("run_id = %q, want run-abc123", parsed.RunID)
	}
}

func TestWriter_DoesNotCollideWhenRunIDsDiffer(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	w := NewWriter(dir)
	start := time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC)
	first, err := w.Write(RunBundle{
		RunID:      "run-one",
		ScenarioID: "same-scenario",
		Adapter:    "cli",
		StartTime:  start,
		Transcript: "first",
	})
	if err != nil {
		t.Fatalf("write first: %v", err)
	}
	second, err := w.Write(RunBundle{
		RunID:      "run-two",
		ScenarioID: "same-scenario",
		Adapter:    "cli",
		StartTime:  start,
		Transcript: "second",
	})
	if err != nil {
		t.Fatalf("write second: %v", err)
	}
	if first.Path == second.Path {
		t.Fatalf("artifact paths collided: %s", first.Path)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read artifact root: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("artifact dir count = %d, want 2", len(entries))
	}
}

func TestWriter_RejectsUnsafeRunIDDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outside := filepath.Join(filepath.Dir(dir), "escape")
	t.Cleanup(func() { _ = os.RemoveAll(outside) })

	w := NewWriter(dir)
	if _, err := w.Write(RunBundle{
		RunID:      "../escape",
		ScenarioID: "unsafe",
		Adapter:    "cli",
		StartTime:  time.Now(),
	}); err == nil {
		t.Fatal("expected unsafe run ID to be rejected")
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatalf("outside path err = %v, want not exist", err)
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

func TestWriter_WritesTimeline(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := NewWriter(dir)
	timeline, _ := json.Marshal(map[string]any{
		"total_steps":    2,
		"mutation_count": 1,
	})
	out, err := w.Write(RunBundle{
		ScenarioID: "test",
		Adapter:    "cli",
		StartTime:  time.Now(),
		Timeline:   timeline,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(out.Path, "timeline.json"))
	if err != nil {
		t.Fatalf("missing timeline.json: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid timeline.json: %v", err)
	}
	if parsed["total_steps"] != float64(2) {
		t.Fatalf("total_steps = %v, want 2", parsed["total_steps"])
	}
}

func TestWriter_WritesScorecard(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	w := NewWriter(dir)
	scorecard := json.RawMessage(`{"score":91,"band":"strong","signals":{"retry_loop":1}}`)

	out, err := w.Write(RunBundle{
		ScenarioID: "test",
		Adapter:    "cli",
		StartTime:  time.Now(),
		Scorecard:  scorecard,
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(out.Path, "scorecard.json"))
	if err != nil {
		t.Fatalf("missing scorecard.json: %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("scorecard.json is not valid JSON: %s", data)
	}
	if string(data) != string(scorecard) {
		t.Fatalf("scorecard = %s, want %s", data, scorecard)
	}
}

func TestWriter_WritesRunErrorAndEvents(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	w := NewWriter(dir)
	runError := json.RawMessage(`{"phase":"agent_run","kind":"adapter_error","message":"adapter exploded"}`)
	runEvents := json.RawMessage(`[{"phase":"run","status":"started"},{"phase":"agent_run","status":"failed"}]`)

	out, err := w.Write(RunBundle{
		ScenarioID: "test",
		Adapter:    "cli",
		StartTime:  time.Now(),
		RunError:   runError,
		RunEvents:  runEvents,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"run-error.json", "run-events.json"} {
		data, err := os.ReadFile(filepath.Join(out.Path, name))
		if err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
		if !json.Valid(data) {
			t.Fatalf("%s is not valid JSON: %s", name, data)
		}
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
		ScenarioID:     "chaos-scenario",
		Adapter:        "cli",
		StartTime:      time.Now(),
		ChaosEnabled:   true,
		ChaosMode:      "once",
		ChaosStepCount: 1,
		ChaosTimeline:  chaosTimeline,
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

func TestWriter_WritesFailureAutopsy(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	w := NewWriter(dir)
	autopsy, _ := json.Marshal(map[string]any{
		"outcome":         "fail",
		"primary_failure": "retry_loop",
	})

	out, err := w.Write(RunBundle{
		ScenarioID: "looping-scenario",
		Adapter:    "cli",
		StartTime:  time.Now(),
		Autopsy:    autopsy,
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(out.Path, "failure-autopsy.json"))
	if err != nil {
		t.Fatalf("missing failure-autopsy.json: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid failure-autopsy.json: %v", err)
	}
	if parsed["primary_failure"] != "retry_loop" {
		t.Fatalf("primary_failure = %v, want retry_loop", parsed["primary_failure"])
	}
}
