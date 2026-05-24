package artifactaudit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	bench "github.com/vitas/evidra-bench/pkg/bench"
)

func TestAnalyzeReportsCompletePassedRun(t *testing.T) {
	t.Parallel()

	runDir := t.TempDir()
	writeCoverageArtifact(t, runDir, "run.json", map[string]any{"scenario_id": "s1"})
	writeCoverageArtifact(t, runDir, "tool-calls.json", []map[string]any{
		{"tool": "run_command", "args": map[string]any{"command": "kubectl get pods"}},
		{"tool": "run_command", "args": map[string]any{"command": "kubectl describe pod/web"}},
	})
	writeCoverageArtifact(t, runDir, "timeline.json", map[string]any{"total_steps": 2, "mutation_count": 0})
	writeCoverageArtifact(t, runDir, "run-events.json", []map[string]any{
		{"phase": "run", "status": "started"},
		{"phase": "run", "status": "completed"},
	})

	result := Analyze([]bench.RunRecord{{
		ID:          "run-1",
		ScenarioID:  "s1",
		Model:       "sonnet",
		Adapter:     "cli",
		Passed:      true,
		ExitCode:    0,
		ArtifactDir: runDir,
		CreatedAt:   time.Now().UTC(),
	}})

	if result.TotalRuns != 1 {
		t.Fatalf("TotalRuns = %d, want 1", result.TotalRuns)
	}
	if result.CompleteRuns != 1 {
		t.Fatalf("CompleteRuns = %d, want 1", result.CompleteRuns)
	}
	if result.CoveragePercent != 100 {
		t.Fatalf("CoveragePercent = %.1f, want 100", result.CoveragePercent)
	}
	if len(result.Findings) != 0 {
		t.Fatalf("findings = %#v, want none", result.Findings)
	}
}

func TestAnalyzeRequiresFailureArtifactsForFailedErrorRuns(t *testing.T) {
	t.Parallel()

	runDir := t.TempDir()
	writeCoverageArtifact(t, runDir, "run.json", map[string]any{"scenario_id": "s1"})
	writeCoverageArtifact(t, runDir, "tool-calls.json", []map[string]any{})
	writeCoverageArtifact(t, runDir, "timeline.json", map[string]any{"total_steps": 0, "mutation_count": 0})
	writeCoverageArtifact(t, runDir, "run-events.json", []map[string]any{
		{"phase": "run", "status": "started"},
		{"phase": "agent_run", "status": "failed"},
	})

	result := Analyze([]bench.RunRecord{{
		ID:          "run-err",
		ScenarioID:  "s1",
		Model:       "sonnet",
		Adapter:     "cli",
		Passed:      false,
		ExitCode:    -1,
		ArtifactDir: runDir,
		CreatedAt:   time.Now().UTC(),
	}})

	if result.CompleteRuns != 0 {
		t.Fatalf("CompleteRuns = %d, want 0", result.CompleteRuns)
	}
	if result.IncompleteRuns != 1 {
		t.Fatalf("IncompleteRuns = %d, want 1", result.IncompleteRuns)
	}
	if got := result.MissingByArtifact["failure-autopsy.json"]; got != 1 {
		t.Fatalf("missing failure-autopsy.json = %d, want 1", got)
	}
	if got := result.MissingByArtifact["run-error.json"]; got != 1 {
		t.Fatalf("missing run-error.json = %d, want 1", got)
	}
	if got := result.MissingByAdapter["cli"]; got != 2 {
		t.Fatalf("missing by adapter cli = %d, want 2", got)
	}
}

func TestAnalyzeFlagsInvalidJSONAndTimelineMismatch(t *testing.T) {
	t.Parallel()

	runDir := t.TempDir()
	writeCoverageArtifact(t, runDir, "run.json", map[string]any{"scenario_id": "s1"})
	writeCoverageArtifact(t, runDir, "tool-calls.json", []map[string]any{{"tool": "run_command"}})
	if err := os.WriteFile(filepath.Join(runDir, "timeline.json"), []byte(`{"total_steps":2}`), 0644); err != nil {
		t.Fatalf("write timeline: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "run-events.json"), []byte(`not-json`), 0644); err != nil {
		t.Fatalf("write run-events: %v", err)
	}

	result := Analyze([]bench.RunRecord{{
		ID:          "run-bad-json",
		ScenarioID:  "s1",
		Model:       "sonnet",
		Adapter:     "cli",
		Passed:      true,
		ArtifactDir: runDir,
		CreatedAt:   time.Now().UTC(),
	}})

	if got := result.InvalidByArtifact["run-events.json"]; got != 1 {
		t.Fatalf("invalid run-events.json = %d, want 1", got)
	}
	if got := result.MismatchByArtifact["timeline.json"]; got != 1 {
		t.Fatalf("timeline mismatches = %d, want 1", got)
	}
	if len(result.Findings) != 2 {
		t.Fatalf("findings = %d, want 2: %#v", len(result.Findings), result.Findings)
	}
}

func writeCoverageArtifact(t *testing.T, dir, name string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", name, err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), append(data, '\n'), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
