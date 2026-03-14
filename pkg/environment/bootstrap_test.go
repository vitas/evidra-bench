package environment

import (
	"strings"
	"testing"
)

func TestDefaultBootstrapPlan_HasSteps(t *testing.T) {
	t.Parallel()
	plan := DefaultBootstrapPlan()
	if len(plan.Steps) == 0 {
		t.Fatal("expected bootstrap steps")
	}
}

func TestDefaultBootstrapPlan_IncludesNamespace(t *testing.T) {
	t.Parallel()
	plan := DefaultBootstrapPlan()
	if !plan.Requires("namespace") {
		t.Fatal("expected namespace bootstrap step")
	}
}

func TestBootstrapStep_KubectlApply(t *testing.T) {
	t.Parallel()
	step := BootstrapStep{
		Name:    "test",
		Type:    StepKubectlApply,
		Path:    "manifests/baseline",
		Feature: "baseline",
	}
	args := step.CommandArgs("/tmp/kubeconfig")
	if len(args) == 0 {
		t.Fatal("expected command args")
	}
	found := false
	for _, a := range args {
		if a == "apply" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected apply in command args")
	}
}

func TestBootstrapStep_Kubectl(t *testing.T) {
	t.Parallel()
	step := BootstrapStep{
		Name: "wait-for-deployment",
		Type: StepKubectl,
		Args: []string{"rollout", "status", "deployment/web", "-n", "bench"},
	}
	args := step.CommandArgs("/tmp/kubeconfig")
	if len(args) < 4 {
		t.Fatalf("unexpected command args: %v", args)
	}
	if args[0] != "kubectl" || args[1] != "--kubeconfig" || args[2] != "/tmp/kubeconfig" {
		t.Fatalf("unexpected command args: %v", args)
	}
}

func TestBootstrapStep_HelmInstallIsIdempotent(t *testing.T) {
	t.Parallel()
	step := BootstrapStep{
		Name:      "install-web",
		Type:      StepHelmInstall,
		Release:   "web",
		Path:      "/repo/charts/web",
		Namespace: "bench",
		Args:      []string{"--create-namespace"},
	}
	args := step.CommandArgs("/tmp/kubeconfig")
	if len(args) == 0 {
		t.Fatal("expected command args")
	}
	got := ""
	for _, arg := range args {
		got += arg + " "
	}
	if got == "" || !containsAll(got, "upgrade", "--install", "web", "/repo/charts/web", "-n", "bench") {
		t.Fatalf("unexpected helm command: %s", got)
	}
}

func containsAll(s string, wants ...string) bool {
	for _, want := range wants {
		if !strings.Contains(s, want) {
			return false
		}
	}
	return true
}
