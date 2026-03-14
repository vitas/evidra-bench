package environment

import (
	"testing"
)

func TestDefaultBootstrapPlan_HasSteps(t *testing.T) {
	t.Parallel()
	plan := DefaultBootstrapPlan()
	if len(plan.Steps) == 0 {
		t.Fatal("expected bootstrap steps")
	}
}

func TestDefaultBootstrapPlan_IncludesArgoCD(t *testing.T) {
	t.Parallel()
	plan := DefaultBootstrapPlan()
	if !plan.Requires("argocd") {
		t.Fatal("expected argocd bootstrap step")
	}
}

func TestDefaultBootstrapPlan_IncludesBaseline(t *testing.T) {
	t.Parallel()
	plan := DefaultBootstrapPlan()
	if !plan.Requires("baseline") {
		t.Fatal("expected baseline bootstrap step")
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
