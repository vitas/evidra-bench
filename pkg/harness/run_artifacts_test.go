package harness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vitas/evidra-bench/pkg/artifact"
	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/environment"
	"github.com/vitas/evidra-bench/pkg/scenario"
	"github.com/vitas/evidra-bench/pkg/store"
)

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
