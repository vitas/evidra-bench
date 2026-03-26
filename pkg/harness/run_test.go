package harness

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"samebits.com/evidra-infra-bench/pkg/adapter"
	"samebits.com/evidra-infra-bench/pkg/agent"
	"samebits.com/evidra-infra-bench/pkg/artifact"
	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/environment"
	"samebits.com/evidra-infra-bench/pkg/scenario"
	"samebits.com/evidra-infra-bench/pkg/store"
	promptdata "samebits.com/evidra/prompts"
)

// fakeProvider is a test double for environment.Provider.
type fakeProvider struct {
	created   bool
	destroyed bool
}

func (f *fakeProvider) Create(_ context.Context, clusterName string) (*environment.Handle, error) {
	f.created = true
	return &environment.Handle{
		ClusterName:    clusterName,
		KubeconfigPath: "/tmp/fake-kubeconfig",
	}, nil
}

func (f *fakeProvider) Destroy(_ context.Context, _ *environment.Handle) error {
	f.destroyed = true
	return nil
}

// fakeAdapter is a test double for adapter.Adapter.
type fakeAdapter struct {
	called bool
}

func (f *fakeAdapter) Run(_ context.Context, _ adapter.RunInput) (*adapter.RunResult, error) {
	f.called = true
	return &adapter.RunResult{
		ExitCode:   0,
		Stdout:     "agent did things",
		Transcript: "agent did things",
		Metadata:   map[string]string{"adapter": "fake"},
	}, nil
}

type sleepingAdapter struct {
	delay time.Duration
}

func (s *sleepingAdapter) Run(ctx context.Context, _ adapter.RunInput) (*adapter.RunResult, error) {
	timer := time.NewTimer(s.delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return &adapter.RunResult{ExitCode: 0, Transcript: "done"}, nil
	}
}

type workspaceCheckingAdapter struct {
	exists bool
}

func (w *workspaceCheckingAdapter) Run(_ context.Context, input adapter.RunInput) (*adapter.RunResult, error) {
	info, err := os.Stat(input.WorkspaceDir)
	if err == nil && info.IsDir() {
		w.exists = true
	}
	return &adapter.RunResult{ExitCode: 0}, nil
}

type fakeRunner struct {
	err error
}

func (f *fakeRunner) Run(_ context.Context, _ *exec.Cmd) ([]byte, error) {
	return nil, f.err
}

type recordingRunner struct {
	commands []string
}

func (r *recordingRunner) Run(_ context.Context, cmd *exec.Cmd) ([]byte, error) {
	r.commands = append(r.commands, strings.Join(cmd.Args, " "))
	return nil, nil
}

func fakeDeps() (Deps, *fakeProvider, *fakeAdapter) {
	fp := &fakeProvider{}
	fa := &fakeAdapter{}
	return Deps{
		EnvProvider: fp,
		Adapter:     fa,
	}, fp, fa
}

func TestHarness_DryRun(t *testing.T) {
	t.Parallel()
	deps, fp, _ := fakeDeps()
	h := New(deps)
	cfg := config.Default()
	cfg.DryRun = true
	cfg.Scenario = "broken-deployment"

	result, err := h.Run(context.Background(), RunRequest{
		Config: cfg,
		Scenario: &scenario.Scenario{
			ID:       "broken-deployment",
			Title:    "Fix broken deployment",
			Category: "kubernetes",
			Prompt:   "prompts/task.md",
			Checks:   []scenario.Check{{Type: "deployment-ready", Namespace: "bench", Name: "web"}},
		},
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if result.ScenarioID != "broken-deployment" {
		t.Fatalf("unexpected scenario: %s", result.ScenarioID)
	}
	if fp.created {
		t.Fatal("environment should not be created in dry-run")
	}
}

func TestHarness_RunSequence(t *testing.T) {
	t.Parallel()
	deps, fp, fa := fakeDeps()
	h := New(deps)
	cfg := config.Default()
	cfg.Scenario = "broken-deployment"
	cfg.Timeout = 10 * time.Second

	result, err := h.Run(context.Background(), RunRequest{
		Config: cfg,
		Scenario: &scenario.Scenario{
			ID:       "broken-deployment",
			Title:    "Fix broken deployment",
			Category: "kubernetes",
			Checks:   []scenario.Check{{Type: "deployment-ready", Namespace: "bench", Name: "web"}},
		},
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if !fp.created {
		t.Fatal("environment should be created")
	}
	if !fp.destroyed {
		t.Fatal("environment should be destroyed")
	}
	if !fa.called {
		t.Fatal("adapter should be called")
	}
	if result.ScenarioID != "broken-deployment" {
		t.Fatalf("unexpected scenario: %s", result.ScenarioID)
	}
}

func TestHarness_ReuseCluster_NoDestroy(t *testing.T) {
	t.Parallel()
	deps, fp, _ := fakeDeps()
	h := New(deps)
	cfg := config.Default()
	cfg.Scenario = "broken-deployment"
	cfg.ReuseCluster = true
	cfg.Timeout = 10 * time.Second

	_, err := h.Run(context.Background(), RunRequest{
		Config: cfg,
		Scenario: &scenario.Scenario{
			ID:       "broken-deployment",
			Title:    "Fix broken deployment",
			Category: "kubernetes",
			Checks:   []scenario.Check{{Type: "deployment-ready", Namespace: "bench", Name: "web"}},
		},
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if fp.destroyed {
		t.Fatal("environment should not be destroyed when reuse-cluster is set")
	}
}

func TestBuildRunMetadata_UsesCanonicalPromptMetadata(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Provider = "claude"
	cfg.Model = "sonnet"
	// Use an embedded prompt path — no filesystem dependency on the parent repo.
	cfg.SystemPromptFile = promptdata.MCPAgentContractPath

	meta := buildRunMetadata(cfg, &agent.LoopResult{
		Turns:        4,
		MemoryWindow: 12,
		TotalUsage: agent.Usage{
			PromptTokens:     120,
			CompletionTokens: 45,
		},
	}, "/tmp/evidence")

	if meta["contract_version"] != promptdata.DefaultContractVersion {
		t.Fatalf("contract_version = %q, want %q", meta["contract_version"], promptdata.DefaultContractVersion)
	}
	expectedSkill := promptdata.DefaultContractSkillVersion
	if meta["skill_version"] != expectedSkill {
		t.Fatalf("skill_version = %q, want %q", meta["skill_version"], expectedSkill)
	}
	if meta["prompt_version"] == "" {
		t.Fatalf("prompt_version is empty")
	}
}

func TestBuildRunMetadata_PrefersExplicitEvidenceMode(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Provider = "claude"
	cfg.Model = "sonnet"
	cfg.EvidenceMode = "none"
	cfg.SmartPrescribe = true
	cfg.ProxyMode = true
	cfg.EvidraBin = "/usr/local/bin/evidra"
	cfg.SystemPromptFile = promptdata.MCPAgentContractPath

	meta := buildRunMetadata(cfg, &agent.LoopResult{}, "/tmp/evidence")

	if meta["evidence_mode"] != "none" {
		t.Fatalf("evidence_mode = %q, want none", meta["evidence_mode"])
	}
}

func TestHarness_StoreUsesExplicitEvidenceMode(t *testing.T) {
	t.Parallel()

	fp := &fakeProvider{}
	fa := &fakeAdapter{}
	storeDir := t.TempDir()
	rs, err := store.Open(storeDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := rs.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
	h := New(Deps{
		EnvProvider: fp,
		Adapter:     fa,
		Store:       rs,
	})

	cfg := config.Default()
	cfg.Scenario = "broken-deployment"
	cfg.RunsDir = filepath.Join(t.TempDir(), "runs")
	cfg.EvidenceMode = "none"
	cfg.MCPServer = "evidra-mcp --signing-mode optional"
	cfg.ProxyMode = true
	cfg.SmartPrescribe = true
	cfg.EvidraBin = "/usr/local/bin/evidra"
	cfg.SystemPromptFile = promptdata.MCPAgentContractPath

	if _, err := h.Run(context.Background(), RunRequest{
		Config: cfg,
		Scenario: &scenario.Scenario{
			ID:       "broken-deployment",
			Title:    "Fix broken deployment",
			Category: "kubernetes",
			Checks:   []scenario.Check{{Type: "deployment-ready", Namespace: "bench", Name: "web"}},
		},
	}); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(storeDir, "results.jsonl"))
	if err != nil {
		t.Fatalf("read jsonl: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 stored record, got %d", len(lines))
	}
	var rec store.RunRecord
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("unmarshal stored record: %v", err)
	}
	if rec.EvidenceMode != "none" {
		t.Fatalf("stored evidence_mode = %q, want none", rec.EvidenceMode)
	}
}

func TestBreakCommandArgs_HelmUpgrade(t *testing.T) {
	t.Parallel()
	s := &scenario.Scenario{
		ID: "helm-failed-upgrade",
		Break: scenario.Break{
			Type:      "helm-upgrade",
			Name:      "web",
			Namespace: "bench",
			Chart:     "/repo/charts/web",
			Path:      "/repo/scenarios/helm/failed-upgrade/fixtures/bad-values.yaml",
		},
	}

	args, err := breakCommandArgs("/tmp/kubeconfig", s)
	if err != nil {
		t.Fatalf("breakCommandArgs failed: %v", err)
	}
	got := strings.Join(args, " ")
	for _, want := range []string{
		"helm",
		"--kubeconfig /tmp/kubeconfig",
		"upgrade web /repo/charts/web",
		"-n bench",
		"-f /repo/scenarios/helm/failed-upgrade/fixtures/bad-values.yaml",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("command %q missing %q", got, want)
		}
	}
}

func TestHarness_ApplyBreak_AllowsExpectedFailure(t *testing.T) {
	t.Parallel()

	h := New(Deps{
		Runner: &fakeRunner{err: errors.New("helm upgrade failed")},
	})

	err := h.applyBreak(context.Background(), "/tmp/kubeconfig", &scenario.Scenario{
		ID: "helm-pending-release",
		Break: scenario.Break{
			Type:         "helm-upgrade",
			Name:         "web",
			Namespace:    "bench",
			Chart:        "/repo/charts/web",
			Path:         "/repo/scenarios/helm/pending-release/fixtures/pending-values.yaml",
			Args:         []string{"--wait", "--timeout", "5s"},
			AllowFailure: true,
		},
	})
	if err != nil {
		t.Fatalf("applyBreak returned error for allowed failure: %v", err)
	}
}

func TestHarness_CreatesRunsDirBeforeAgent(t *testing.T) {
	t.Parallel()

	fp := &fakeProvider{}
	wa := &workspaceCheckingAdapter{}
	h := New(Deps{
		EnvProvider: fp,
		Adapter:     wa,
	})

	cfg := config.Default()
	cfg.Scenario = "broken-deployment"
	cfg.RunsDir = filepath.Join(t.TempDir(), "runs")

	if _, err := h.Run(context.Background(), RunRequest{
		Config: cfg,
		Scenario: &scenario.Scenario{
			ID:       "broken-deployment",
			Title:    "Fix broken deployment",
			Category: "kubernetes",
		},
	}); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if !wa.exists {
		t.Fatal("workspace directory was not created before agent run")
	}
}

func TestHarness_RunExecutesAfterBreakSteps(t *testing.T) {
	t.Parallel()

	fp := &fakeProvider{}
	runner := &recordingRunner{}
	h := New(Deps{
		EnvProvider:  fp,
		Bootstrapper: environment.NewBootstrapper(runner),
		Runner:       runner,
		Adapter:      &fakeAdapter{},
	})

	cfg := config.Default()
	cfg.Scenario = "crashloop-backoff"
	cfg.RunsDir = filepath.Join(t.TempDir(), "runs")

	_, err := h.Run(context.Background(), RunRequest{
		Config: cfg,
		Scenario: &scenario.Scenario{
			ID:       "crashloop-backoff",
			Title:    "Fix a pod stuck in CrashLoopBackOff",
			Category: "kubernetes",
			Break: scenario.Break{
				Type: "kubectl-apply",
				Path: "/repo/scenarios/kubernetes/crashloop-backoff/fixtures/broken.yaml",
			},
			AfterBreak: []scenario.BootstrapStep{
				{
					Name: "wait-for-broken-rollout",
					Type: "kubectl",
					Args: []string{"wait", "--for=jsonpath={.status.availableReplicas}=0", "deployment/app", "-n", "bench", "--timeout=30s"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if len(runner.commands) < 2 {
		t.Fatalf("expected break command to run before after_break steps, got %v", runner.commands)
	}
	if !strings.Contains(runner.commands[len(runner.commands)-2], "apply -f /repo/scenarios/kubernetes/crashloop-backoff/fixtures/broken.yaml") {
		t.Fatalf("unexpected recorded commands: %v", runner.commands)
	}
	if !strings.Contains(runner.commands[len(runner.commands)-1], "wait --for=jsonpath={.status.availableReplicas}=0 deployment/app -n bench --timeout=30s") {
		t.Fatalf("unexpected recorded commands: %v", runner.commands)
	}
}

func TestProviderEvidenceDir_IsIsolatedPerRun(t *testing.T) {
	t.Parallel()

	runsDir := t.TempDir()
	a := providerEvidenceDir("", runsDir, "broken-deployment", time.Unix(100, 0))
	b := providerEvidenceDir("", runsDir, "broken-deployment", time.Unix(101, 0))

	shared := filepath.Join(runsDir, "evidence")
	if a == shared {
		t.Fatalf("provider evidence dir = %q, want per-run path", a)
	}
	if a == b {
		t.Fatalf("provider evidence dirs should differ per run: %q", a)
	}
}

func TestProviderToolCalls_ExportsStructuredResults(t *testing.T) {
	t.Parallel()

	messages := []agent.Message{
		{
			Role: "assistant",
			ToolCalls: []agent.ToolCall{
				{
					ID:        "call-1",
					Name:      "run_command",
					Arguments: `{"command":"kubectl get pods -n bench"}`,
				},
				{
					ID:        "call-2",
					Name:      "evidra_report",
					Arguments: `{"prescription_id":"p1","verdict":"success","exit_code":0}`,
				},
			},
		},
		{Role: "tool", ToolCallID: "call-1", Content: "pod/web Ready"},
		{Role: "tool", ToolCallID: "call-2", Content: "reported"},
	}

	got := providerToolCalls(messages)
	if len(got) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(got))
	}
	if got[0].Tool != "run_command" {
		t.Fatalf("first tool = %q, want run_command", got[0].Tool)
	}
	if got[0].Result != "pod/web Ready" {
		t.Fatalf("first result = %q, want tool output", got[0].Result)
	}
	if got[0].Args["command"] != "kubectl get pods -n bench" {
		t.Fatalf("unexpected args: %#v", got[0].Args)
	}
	if got[1].Tool != "evidra_report" {
		t.Fatalf("second tool = %q, want evidra_report", got[1].Tool)
	}
	if got[1].Args["prescription_id"] != "p1" {
		t.Fatalf("unexpected second args: %#v", got[1].Args)
	}
}

func TestHarness_RunExecutesChaosStepsDuringAgent(t *testing.T) {
	t.Parallel()

	fp := &fakeProvider{}
	runner := &recordingRunner{}
	h := New(Deps{
		EnvProvider: fp,
		Runner:      runner,
		Adapter:     &sleepingAdapter{delay: 40 * time.Millisecond},
	})

	cfg := config.Default()
	cfg.Scenario = "pod-kill-during-repair"
	cfg.RunsDir = filepath.Join(t.TempDir(), "runs")

	_, err := h.Run(context.Background(), RunRequest{
		Config: cfg,
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
	if len(runner.commands) != 1 {
		t.Fatalf("expected 1 chaos command, got %v", runner.commands)
	}
	if !strings.Contains(runner.commands[0], "kubectl --kubeconfig /tmp/fake-kubeconfig delete pod -n bench web-0") {
		t.Fatalf("unexpected chaos command: %v", runner.commands)
	}
}

func TestHarness_ChaosStopsWhenAgentDone(t *testing.T) {
	t.Parallel()

	fp := &fakeProvider{}
	runner := &recordingRunner{}
	h := New(Deps{
		EnvProvider: fp,
		Runner:      runner,
		Adapter:     &sleepingAdapter{delay: 20 * time.Millisecond},
	})

	cfg := config.Default()
	cfg.Scenario = "pod-kill-during-repair"
	cfg.RunsDir = filepath.Join(t.TempDir(), "runs")

	_, err := h.Run(context.Background(), RunRequest{
		Config: cfg,
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
	if len(runner.commands) != 1 {
		t.Fatalf("expected pending chaos to be canceled, got %v", runner.commands)
	}
}

func TestHarness_ChaosRepeatModeReplaysSteps(t *testing.T) {
	t.Parallel()

	fp := &fakeProvider{}
	runner := &recordingRunner{}
	h := New(Deps{
		EnvProvider: fp,
		Runner:      runner,
		Adapter:     &sleepingAdapter{delay: 35 * time.Millisecond},
	})

	cfg := config.Default()
	cfg.Scenario = "pod-kill-during-repair"
	cfg.RunsDir = filepath.Join(t.TempDir(), "runs")

	_, err := h.Run(context.Background(), RunRequest{
		Config: cfg,
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
		EnvProvider: fp,
		Runner:      runner,
		Adapter:     &sleepingAdapter{delay: 25 * time.Millisecond},
		Writer:      writer,
	})

	cfg := config.Default()
	cfg.Scenario = "pod-kill-during-repair"
	cfg.RunsDir = filepath.Join(t.TempDir(), "runs")

	result, err := h.Run(context.Background(), RunRequest{
		Config: cfg,
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
