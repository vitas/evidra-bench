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
	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/environment"
	"samebits.com/evidra-infra-bench/pkg/scenario"
	"samebits.com/evidra-infra-bench/pkg/store"
)

// fakeProvider is a test double for environment.ClusterLifecycle.
type fakeProvider struct {
	created   bool
	destroyed bool
}

func (f *fakeProvider) Create(_ context.Context, clusterName string, _ environment.ClusterSpec) (*environment.Handle, error) {
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

func (f *fakeProvider) Recreate(_ context.Context, clusterName string, _ environment.ClusterSpec) (*environment.Handle, error) {
	return &environment.Handle{ClusterName: clusterName, KubeconfigPath: "/tmp/fake-kubeconfig"}, nil
}

func (f *fakeProvider) HealthCheck(_ context.Context, _ string) error { return nil }

func (f *fakeProvider) ForceDeleteNamespace(_ context.Context, _, _ string) error { return nil }

func (f *fakeProvider) CreateNamespace(_ context.Context, _, _ string) error { return nil }

func (f *fakeProvider) RunCanary(_ context.Context, _, _ string) error { return nil }

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

type autopsyAdapter struct{}

func (a *autopsyAdapter) Run(_ context.Context, _ adapter.RunInput) (*adapter.RunResult, error) {
	return &adapter.RunResult{
		ExitCode:   0,
		Transcript: "The deployment is fixed and everything is working.",
		ToolCalls: []adapter.ToolCallRecord{
			{
				Tool:   "run_command",
				Args:   map[string]any{"command": "kubectl get pods -n bench"},
				Result: "web 0/1 ErrImagePull",
			},
			{
				Tool:   "run_command",
				Args:   map[string]any{"command": "kubectl get pods -n bench"},
				Result: "web 0/1 ErrImagePull",
			},
			{
				Tool:   "run_command",
				Args:   map[string]any{"command": "kubectl get pods -n bench"},
				Result: "web 0/1 ErrImagePull",
			},
		},
		Metadata: map[string]string{"turns": "3", "prompt_tokens": "100", "completion_tokens": "50"},
	}, nil
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

// fakeKubeconfig creates a temporary kubeconfig file and returns its path.
// The file must exist on disk for the harness to pass its stat check.
func fakeKubeconfig(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "kubeconfig-*")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func writePromptMetadataFile(t *testing.T, contractVersion, promptVersion string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "prompt.md")
	body := "<!-- contract: " + contractVersion + " -->\n" +
		"<!-- prompt: " + promptVersion + " -->\n" +
		"You are an infra agent.\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
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

func TestHarness_Run_RequiresKubeconfigWhenNotDryRun(t *testing.T) {
	t.Parallel()

	h := New(Deps{EnvProvider: &fakeProvider{}})
	_, err := h.Run(context.Background(), RunRequest{
		Config:   config.Config{},
		Scenario: &scenario.Scenario{ID: "test/missing-kubeconfig"},
		// KubeconfigPath intentionally empty.
	})
	if err == nil {
		t.Fatal("expected error when KubeconfigPath is empty and not dry-run")
	}
	if !strings.Contains(err.Error(), "kubeconfig path is required") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestHarness_Run_DoesNotCreateOrDestroyEnvironmentWhenLeaseProvided(t *testing.T) {
	t.Parallel()

	kubeconfig := fakeKubeconfig(t)
	deps, fp, fa := fakeDeps()
	h := New(deps)
	cfg := config.Default()
	cfg.Scenario = "broken-deployment"
	cfg.Timeout = 10 * time.Second

	result, err := h.Run(context.Background(), RunRequest{
		Config:         cfg,
		KubeconfigPath: kubeconfig,
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
	if fp.created {
		t.Fatal("Create must not be called when a leased kubeconfig is provided")
	}
	if fp.destroyed {
		t.Fatal("Destroy must not be called when a leased kubeconfig is provided")
	}
	if !fa.called {
		t.Fatal("adapter should be called")
	}
	if result.ScenarioID != "broken-deployment" {
		t.Fatalf("unexpected scenario: %s", result.ScenarioID)
	}
}

func TestBuildRunMetadata_UsesPromptFileMetadata(t *testing.T) {
	t.Parallel()

	promptPath := writePromptMetadataFile(t, "v1.2.3", "p7")
	cfg := config.Default()
	cfg.Provider = "claude"
	cfg.Model = "sonnet"
	cfg.SystemPromptFile = promptPath

	meta := buildRunMetadata(cfg, &agent.LoopResult{
		Turns:        4,
		MemoryWindow: 12,
		TotalUsage: agent.Usage{
			PromptTokens:     120,
			CompletionTokens: 45,
		},
	}, "/tmp/evidence")

	if meta["contract_version"] != "v1.2.3" {
		t.Fatalf("contract_version = %q, want v1.2.3", meta["contract_version"])
	}
	if meta["skill_version"] != "1.2" {
		t.Fatalf("skill_version = %q, want 1.2", meta["skill_version"])
	}
	if meta["prompt_version"] != "p7" {
		t.Fatalf("prompt_version = %q, want p7", meta["prompt_version"])
	}
}

func TestBuildRunMetadata_PrefersExplicitEvidenceMode(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Provider = "claude"
	cfg.Model = "sonnet"
	cfg.EvidenceMode = "none"
	cfg.MCPServer = "evidra-mcp --signing-mode optional"
	cfg.SystemPromptFile = writePromptMetadataFile(t, "v1.2.3", "p7")
	cfg.ContractVersion = "v9.9.9"

	meta := buildRunMetadata(cfg, &agent.LoopResult{}, "/tmp/evidence")

	if meta["evidence_mode"] != "none" {
		t.Fatalf("evidence_mode = %q, want none", meta["evidence_mode"])
	}
	if _, ok := meta["contract_version"]; ok {
		t.Fatalf("contract_version unexpectedly present: %q", meta["contract_version"])
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
	cfg.SystemPromptFile = writePromptMetadataFile(t, "v1.2.3", "p7")

	if _, err := h.Run(context.Background(), RunRequest{
		Config:         cfg,
		KubeconfigPath: fakeKubeconfig(t),
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

// TestHarness_Run_LeaseEnvIsAvailableToCloudSetup verifies that ExtraEnv vars
// (e.g. AWS credentials from a profile lease) are propagated to the scenario's
// cloud.setup script. This protects the aws-localstack workflow where the
// provisioner writes lease.env and the harness forwards it to scenario scripts.
func TestHarness_Run_LeaseEnvIsAvailableToCloudSetup(t *testing.T) {
	t.Parallel()

	// Create a setup script that writes received env vars to a marker file.
	tmpDir := t.TempDir()
	markerFile := filepath.Join(tmpDir, "env-marker.txt")
	setupScript := filepath.Join(tmpDir, "setup.sh")
	if err := os.WriteFile(setupScript, []byte(
		"#!/bin/sh\nset -eu\necho \"AWS_ENDPOINT_URL=$AWS_ENDPOINT_URL\" > "+markerFile+"\n"+
			"echo \"AWS_ACCESS_KEY_ID=$AWS_ACCESS_KEY_ID\" >> "+markerFile+"\n",
	), 0o755); err != nil {
		t.Fatal(err)
	}

	fp := &fakeProvider{}
	fa := &fakeAdapter{}
	h := New(Deps{
		EnvProvider: fp,
		Adapter:     fa,
	})

	cfg := config.Default()
	cfg.Scenario = "aws/s3-bucket-policy"
	cfg.RunsDir = filepath.Join(t.TempDir(), "runs")

	_, err := h.Run(context.Background(), RunRequest{
		Config:         cfg,
		KubeconfigPath: fakeKubeconfig(t),
		ExtraEnv: []string{
			"AWS_ENDPOINT_URL=http://localhost:4566",
			"AWS_ACCESS_KEY_ID=test-key",
		},
		Scenario: &scenario.Scenario{
			ID:       "s3-bucket-policy",
			Title:    "Fix S3 bucket policy",
			Category: "aws",
			Environment: scenario.EnvironmentConfig{
				Cloud: scenario.CloudConfig{
					Provider: "localstack",
					Services: []string{"s3", "iam"},
					Setup:    setupScript,
				},
			},
			Checks: []scenario.Check{{Type: "command-succeeds", Name: "verify", Condition: "true"}},
		},
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Verify the setup script received the lease env vars.
	data, readErr := os.ReadFile(markerFile)
	if readErr != nil {
		t.Fatalf("setup script did not write marker file: %v", readErr)
	}
	content := string(data)
	if !strings.Contains(content, "AWS_ENDPOINT_URL=http://localhost:4566") {
		t.Fatalf("AWS_ENDPOINT_URL not propagated to cloud setup script; marker:\n%s", content)
	}
	if !strings.Contains(content, "AWS_ACCESS_KEY_ID=test-key") {
		t.Fatalf("AWS_ACCESS_KEY_ID not propagated to cloud setup script; marker:\n%s", content)
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
		Bootstrapper: environment.NewBootstrapper(&fakeRunner{err: errors.New("helm upgrade failed")}),
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
		Config:         cfg,
		KubeconfigPath: fakeKubeconfig(t),
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
		Adapter:      &fakeAdapter{},
	})

	cfg := config.Default()
	cfg.Scenario = "crashloop-backoff"
	cfg.RunsDir = filepath.Join(t.TempDir(), "runs")

	kubeconfig := fakeKubeconfig(t)
	_, err := h.Run(context.Background(), RunRequest{
		Config:         cfg,
		KubeconfigPath: kubeconfig,
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
