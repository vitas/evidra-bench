package harness

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

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
