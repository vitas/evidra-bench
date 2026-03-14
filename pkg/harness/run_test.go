package harness

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"samebits.com/evidra-infra-bench/pkg/adapter"
	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/environment"
	"samebits.com/evidra-infra-bench/pkg/scenario"
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
