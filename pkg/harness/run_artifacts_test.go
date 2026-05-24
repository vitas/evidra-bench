package harness

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vitas/evidra-bench/pkg/adapter"
	"github.com/vitas/evidra-bench/pkg/artifact"
	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/environment"
	"github.com/vitas/evidra-bench/pkg/scenario"
	"github.com/vitas/evidra-bench/pkg/store"
)

func TestHarness_RunWritesFailureArtifactsWhenAdapterErrors(t *testing.T) {
	t.Parallel()

	artifactRoot := t.TempDir()
	h := New(Deps{
		EnvProvider: &fakeProvider{},
		Adapter:     &failingAdapter{err: errors.New("adapter exploded")},
		Writer:      artifact.NewWriter(artifactRoot),
	})

	cfg := config.Default()
	cfg.Scenario = "adapter-error"
	cfg.RunsDir = filepath.Join(t.TempDir(), "runs")

	_, err := h.Run(context.Background(), RunRequest{
		Config:         cfg,
		KubeconfigPath: fakeKubeconfig(t),
		Scenario: &scenario.Scenario{
			ID:       "adapter-error",
			Title:    "Adapter error",
			Category: "kubernetes",
		},
	})
	if err == nil {
		t.Fatal("expected adapter error")
	}

	runDir := singleArtifactDir(t, artifactRoot)
	for _, name := range []string{"tool-calls.json", "timeline.json", "failure-autopsy.json", "run-error.json", "run-events.json"} {
		if _, err := os.Stat(filepath.Join(runDir, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}

	var toolCalls []adapter.ToolCallRecord
	readArtifactJSON(t, runDir, "tool-calls.json", &toolCalls)
	if len(toolCalls) != 0 {
		t.Fatalf("tool calls = %d, want 0", len(toolCalls))
	}

	var timeline struct {
		TotalSteps    int `json:"total_steps"`
		MutationCount int `json:"mutation_count"`
	}
	readArtifactJSON(t, runDir, "timeline.json", &timeline)
	if timeline.TotalSteps != 0 || timeline.MutationCount != 0 {
		t.Fatalf("timeline = %+v, want empty failed-run timeline", timeline)
	}

	var runErr struct {
		Phase    string `json:"phase"`
		Kind     string `json:"kind"`
		Message  string `json:"message"`
		ExitCode int    `json:"exit_code"`
	}
	readArtifactJSON(t, runDir, "run-error.json", &runErr)
	if runErr.Phase != "agent_run" {
		t.Fatalf("run-error phase = %q, want agent_run", runErr.Phase)
	}
	if runErr.Kind != "adapter_error" {
		t.Fatalf("run-error kind = %q, want adapter_error", runErr.Kind)
	}
	if !strings.Contains(runErr.Message, "adapter exploded") {
		t.Fatalf("run-error message = %q, want adapter exploded", runErr.Message)
	}
	if runErr.ExitCode != -1 {
		t.Fatalf("run-error exit_code = %d, want -1", runErr.ExitCode)
	}

	var events []struct {
		Phase  string `json:"phase"`
		Status string `json:"status"`
	}
	readArtifactJSON(t, runDir, "run-events.json", &events)
	if len(events) < 2 {
		t.Fatalf("events = %d, want at least start and failed", len(events))
	}
	if events[len(events)-1].Status != "failed" {
		t.Fatalf("last event status = %q, want failed", events[len(events)-1].Status)
	}

	var autopsy struct {
		Outcome        string `json:"outcome"`
		PrimaryFailure string `json:"primary_failure"`
		Summary        string `json:"summary"`
	}
	readArtifactJSON(t, runDir, "failure-autopsy.json", &autopsy)
	if autopsy.Outcome != "fail" {
		t.Fatalf("autopsy outcome = %q, want fail", autopsy.Outcome)
	}
	if autopsy.PrimaryFailure != "run_error" {
		t.Fatalf("autopsy primary_failure = %q, want run_error", autopsy.PrimaryFailure)
	}
	if !strings.Contains(autopsy.Summary, "agent_run") {
		t.Fatalf("autopsy summary = %q, want phase context", autopsy.Summary)
	}
}

func TestHarness_RunPreservesToolCallsWhenVerifierErrors(t *testing.T) {
	t.Parallel()

	artifactRoot := t.TempDir()
	h := New(Deps{
		EnvProvider: &fakeProvider{},
		Adapter:     &autopsyAdapter{},
		Writer:      artifact.NewWriter(artifactRoot),
	})

	cfg := config.Default()
	cfg.Scenario = "verifier-error"
	cfg.RunsDir = filepath.Join(t.TempDir(), "runs")

	_, err := h.Run(context.Background(), RunRequest{
		Config:         cfg,
		KubeconfigPath: fakeKubeconfig(t),
		Scenario: &scenario.Scenario{
			ID:       "verifier-error",
			Title:    "Verifier error",
			Category: "kubernetes",
			Checks:   []scenario.Check{{Type: "unknown-check", Name: "web"}},
		},
	})
	if err == nil {
		t.Fatal("expected verifier error")
	}

	runDir := singleArtifactDir(t, artifactRoot)

	var toolCalls []adapter.ToolCallRecord
	readArtifactJSON(t, runDir, "tool-calls.json", &toolCalls)
	if len(toolCalls) == 0 {
		t.Fatal("tool calls were not preserved after verifier error")
	}

	var timeline struct {
		TotalSteps    int `json:"total_steps"`
		MutationCount int `json:"mutation_count"`
	}
	readArtifactJSON(t, runDir, "timeline.json", &timeline)
	if timeline.TotalSteps == 0 {
		t.Fatal("timeline total_steps = 0, want preserved adapter tool calls")
	}
	if timeline.MutationCount == 0 {
		t.Fatal("timeline mutation_count = 0, want preserved mutation")
	}

	var runErr struct {
		Phase string `json:"phase"`
		Kind  string `json:"kind"`
	}
	readArtifactJSON(t, runDir, "run-error.json", &runErr)
	if runErr.Phase != "verification" {
		t.Fatalf("run-error phase = %q, want verification", runErr.Phase)
	}
	if runErr.Kind != "verifier_error" {
		t.Fatalf("run-error kind = %q, want verifier_error", runErr.Kind)
	}
}

func TestHarness_RunStoresFailedRecordWhenAdapterErrors(t *testing.T) {
	t.Parallel()

	resultsStore, err := store.Open(filepath.Join(t.TempDir(), "store"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = resultsStore.Close() }()

	artifactRoot := t.TempDir()
	h := New(Deps{
		EnvProvider: &fakeProvider{},
		Adapter:     &failingAdapter{err: errors.New("adapter exploded")},
		Writer:      artifact.NewWriter(artifactRoot),
		Store:       resultsStore,
	})

	cfg := config.Default()
	cfg.Scenario = "adapter-error-store"
	cfg.RunsDir = filepath.Join(t.TempDir(), "runs")

	_, err = h.Run(context.Background(), RunRequest{
		Config:         cfg,
		KubeconfigPath: fakeKubeconfig(t),
		Scenario: &scenario.Scenario{
			ID:       "adapter-error-store",
			Title:    "Adapter error",
			Category: "kubernetes",
		},
	})
	if err == nil {
		t.Fatal("expected adapter error")
	}

	records, err := resultsStore.Query(store.QueryFilters{ScenarioID: "adapter-error-store"})
	if err != nil {
		t.Fatalf("query store: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	if records[0].Passed {
		t.Fatal("stored record passed = true, want false")
	}
	if records[0].ArtifactDir == "" {
		t.Fatal("stored record artifact_dir is empty")
	}
	if records[0].ExitCode != -1 {
		t.Fatalf("stored record exit_code = %d, want -1", records[0].ExitCode)
	}
}

func TestHarness_RunExecutesChaosStepsDuringAgent(t *testing.T) {
	t.Parallel()

	fp := &fakeProvider{}
	runner := &recordingRunner{}
	h := New(Deps{
		EnvProvider:  fp,
		Bootstrapper: environment.NewBootstrapper(runner),
		Adapter:      &sleepingAdapter{delay: 40 * time.Millisecond},
	})

	cfg := config.Default()
	cfg.Scenario = "pod-kill-during-repair"
	cfg.RunsDir = filepath.Join(t.TempDir(), "runs")

	kubeconfig := fakeKubeconfig(t)
	_, err := h.Run(context.Background(), RunRequest{
		Config:         cfg,
		KubeconfigPath: kubeconfig,
		Scenario: &scenario.Scenario{
			ID:       "pod-kill-during-repair",
			Title:    "Pod kill during repair",
			Category: "kubernetes",
			Checks:   []scenario.Check{{Type: "deployment-ready", Namespace: "bench", Name: "web"}},
			Chaos: scenario.ChaosConfig{
				StopOnAgentDone: true,
				Steps: []scenario.ChaosStep{
					{
						Name: "kill-web",
						Type: "kubectl",
						At:   scenario.Duration{Duration: 5 * time.Millisecond, Set: true},
						Args: []string{"delete", "pod", "-n", "bench", "web-0"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	// Find the chaos delete command among recorded commands (bootstrap commands may precede it).
	found := false
	for _, cmd := range runner.commands {
		if strings.Contains(cmd, "kubectl --kubeconfig "+kubeconfig+" delete pod -n bench web-0") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("chaos delete command not found in: %v", runner.commands)
	}
}

func TestHarness_ChaosStopsWhenAgentDone(t *testing.T) {
	t.Parallel()

	fp := &fakeProvider{}
	runner := &recordingRunner{}
	h := New(Deps{
		EnvProvider:  fp,
		Bootstrapper: environment.NewBootstrapper(runner),
		Adapter:      &sleepingAdapter{delay: 20 * time.Millisecond},
	})

	cfg := config.Default()
	cfg.Scenario = "pod-kill-during-repair"
	cfg.RunsDir = filepath.Join(t.TempDir(), "runs")

	_, err := h.Run(context.Background(), RunRequest{
		Config:         cfg,
		KubeconfigPath: fakeKubeconfig(t),
		Scenario: &scenario.Scenario{
			ID:       "pod-kill-during-repair",
			Title:    "Pod kill during repair",
			Category: "kubernetes",
			Checks:   []scenario.Check{{Type: "deployment-ready", Namespace: "bench", Name: "web"}},
			Chaos: scenario.ChaosConfig{
				StopOnAgentDone: true,
				Steps: []scenario.ChaosStep{
					{
						Name: "kill-web",
						Type: "kubectl",
						At:   scenario.Duration{Duration: 5 * time.Millisecond, Set: true},
						Args: []string{"delete", "pod", "-n", "bench", "web-0"},
					},
					{
						Name: "kill-web-again",
						Type: "kubectl",
						At:   scenario.Duration{Duration: 60 * time.Millisecond, Set: true},
						Args: []string{"delete", "pod", "-n", "bench", "web-1"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	// Count chaos delete commands — only the first (at 5ms) should execute;
	// the second (at 60ms) should be canceled because the agent finishes at 20ms.
	chaosCount := 0
	for _, cmd := range runner.commands {
		if strings.Contains(cmd, "delete pod") {
			chaosCount++
		}
	}
	if chaosCount != 1 {
		t.Fatalf("expected 1 chaos command (second should be canceled), got %d: %v", chaosCount, runner.commands)
	}
}

func TestHarness_ChaosRepeatModeReplaysSteps(t *testing.T) {
	t.Parallel()

	fp := &fakeProvider{}
	runner := &recordingRunner{}
	h := New(Deps{
		EnvProvider:  fp,
		Bootstrapper: environment.NewBootstrapper(runner),
		Adapter:      &sleepingAdapter{delay: 35 * time.Millisecond},
	})

	cfg := config.Default()
	cfg.Scenario = "pod-kill-during-repair"
	cfg.RunsDir = filepath.Join(t.TempDir(), "runs")

	_, err := h.Run(context.Background(), RunRequest{
		Config:         cfg,
		KubeconfigPath: fakeKubeconfig(t),
		Scenario: &scenario.Scenario{
			ID:       "pod-kill-during-repair",
			Title:    "Pod kill during repair",
			Category: "kubernetes",
			Checks:   []scenario.Check{{Type: "deployment-ready", Namespace: "bench", Name: "web"}},
			Chaos: scenario.ChaosConfig{
				Mode:            "repeat",
				StopOnAgentDone: true,
				Steps: []scenario.ChaosStep{
					{
						Name: "kill-web",
						Type: "kubectl",
						At:   scenario.Duration{Duration: 5 * time.Millisecond, Set: true},
						Args: []string{"delete", "pod", "-n", "bench", "web-0"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if len(runner.commands) < 2 {
		t.Fatalf("expected repeated chaos commands, got %v", runner.commands)
	}
}

func TestHarness_RunWritesChaosArtifacts(t *testing.T) {
	t.Parallel()

	fp := &fakeProvider{}
	runner := &recordingRunner{}
	writer := artifact.NewWriter(t.TempDir())
	h := New(Deps{
		EnvProvider:  fp,
		Bootstrapper: environment.NewBootstrapper(runner),
		Adapter:      &sleepingAdapter{delay: 25 * time.Millisecond},
		Writer:       writer,
	})

	cfg := config.Default()
	cfg.Scenario = "pod-kill-during-repair"
	cfg.RunsDir = filepath.Join(t.TempDir(), "runs")

	result, err := h.Run(context.Background(), RunRequest{
		Config:         cfg,
		KubeconfigPath: fakeKubeconfig(t),
		Scenario: &scenario.Scenario{
			ID:       "pod-kill-during-repair",
			Title:    "Pod kill during repair",
			Category: "kubernetes",
			Checks:   []scenario.Check{{Type: "deployment-ready", Namespace: "bench", Name: "web"}},
			Chaos: scenario.ChaosConfig{
				StopOnAgentDone: true,
				Steps: []scenario.ChaosStep{
					{
						Name: "kill-web",
						Type: "kubectl",
						At:   scenario.Duration{Duration: 5 * time.Millisecond, Set: true},
						Args: []string{"delete", "pod", "-n", "bench", "web-0"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(result.ArtifactDir, "run.json"))
	if err != nil {
		t.Fatalf("read run.json: %v", err)
	}
	var parsed struct {
		ChaosEnabled   bool `json:"chaos_enabled"`
		ChaosStepCount int  `json:"chaos_step_count"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse run.json: %v", err)
	}
	if !parsed.ChaosEnabled {
		t.Fatal("run.json chaos_enabled = false, want true")
	}
	if parsed.ChaosStepCount != 1 {
		t.Fatalf("run.json chaos_step_count = %d, want 1", parsed.ChaosStepCount)
	}
	if _, err := os.Stat(filepath.Join(result.ArtifactDir, "chaos.json")); err != nil {
		t.Fatalf("missing chaos.json: %v", err)
	}
}

func TestHarness_RunWritesFailureAutopsyArtifact(t *testing.T) {
	t.Parallel()

	fp := &fakeProvider{}
	writer := artifact.NewWriter(t.TempDir())
	h := New(Deps{
		EnvProvider: fp,
		Adapter:     &autopsyAdapter{},
		Writer:      writer,
	})

	cfg := config.Default()
	cfg.Scenario = "broken-deployment"
	cfg.RunsDir = filepath.Join(t.TempDir(), "runs")

	result, err := h.Run(context.Background(), RunRequest{
		Config:         cfg,
		KubeconfigPath: fakeKubeconfig(t),
		Scenario: &scenario.Scenario{
			ID:       "broken-deployment",
			Title:    "Broken deployment",
			Category: "kubernetes",
			Checks:   []scenario.Check{{Type: "deployment-ready", Namespace: "bench", Name: "web"}},
		},
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(result.ArtifactDir, "failure-autopsy.json"))
	if err != nil {
		t.Fatalf("read failure-autopsy.json: %v", err)
	}
	var parsed struct {
		Outcome        string `json:"outcome"`
		PrimaryFailure string `json:"primary_failure"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse failure-autopsy.json: %v", err)
	}
	if parsed.Outcome != "fail" {
		t.Fatalf("outcome = %q, want fail", parsed.Outcome)
	}
	if parsed.PrimaryFailure != "premature_success" {
		t.Fatalf("primary_failure = %q, want premature_success", parsed.PrimaryFailure)
	}

	timelineData, err := os.ReadFile(filepath.Join(result.ArtifactDir, "timeline.json"))
	if err != nil {
		t.Fatalf("read timeline.json: %v", err)
	}
	var timeline struct {
		TotalSteps    int `json:"total_steps"`
		MutationCount int `json:"mutation_count"`
	}
	if err := json.Unmarshal(timelineData, &timeline); err != nil {
		t.Fatalf("parse timeline.json: %v", err)
	}
	if timeline.TotalSteps == 0 {
		t.Fatal("timeline total_steps = 0, want recorded tool calls")
	}
	if timeline.MutationCount == 0 {
		t.Fatal("timeline mutation_count = 0, want recorded mutation")
	}
}

func TestBuildFailureAutopsyJSONUsesScenarioHints(t *testing.T) {
	t.Parallel()

	toolCallsJSON := json.RawMessage(`[{
		"tool": "run_command",
		"args": {"command": "kubectl delete namespace bench"},
		"result": "namespace/bench deleted"
	}]`)
	hints := scenario.AutopsyHints{
		ForbiddenActions: []scenario.AutopsyPattern{
			{Kind: "command_pattern", Pattern: "kubectl delete namespace", Severity: "critical"},
		},
	}

	data := buildFailureAutopsyJSON(store.RunRecord{Passed: false}, toolCallsJSON, "", nil, hints)
	if len(data) == 0 {
		t.Fatal("autopsy JSON is empty")
	}

	var parsed struct {
		PrimaryFailure string `json:"primary_failure"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse autopsy JSON: %v", err)
	}
	if parsed.PrimaryFailure != "unsafe_action" {
		t.Fatalf("primary_failure = %q, want unsafe_action", parsed.PrimaryFailure)
	}
}

func singleArtifactDir(t *testing.T, root string) string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read artifact root: %v", err)
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, filepath.Join(root, entry.Name()))
		}
	}
	if len(dirs) != 1 {
		t.Fatalf("artifact dirs = %v, want exactly one", dirs)
	}
	return dirs[0]
}

func readArtifactJSON(t *testing.T, runDir, name string, out any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(runDir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
}
