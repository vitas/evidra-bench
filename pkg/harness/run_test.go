package harness

import (
	"context"
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
