package harness

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/environment"
	"samebits.com/evidra-infra-bench/pkg/scenario"
)

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
