package scenario

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestStage_UnmarshalYAML(t *testing.T) {
	t.Parallel()
	input := `
name: surface
break:
  type: kubectl-apply
  path: fixtures/broken.yaml
  memory: compact
verify:
  - type: deployment-ready
    namespace: bench
    name: web
agent_goal: "New issue detected"
on_fail: stop
timeout: 3m
trap:
  name: delete-policy
  detect: resource-deleted(networkpolicy/api-db)
  points: 10
`
	var s Stage
	if err := yaml.Unmarshal([]byte(input), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.Name != "surface" {
		t.Errorf("Name = %q, want surface", s.Name)
	}
	if s.Break.Memory != "compact" {
		t.Errorf("Break.Memory = %q, want compact", s.Break.Memory)
	}
	if s.AgentGoal != "New issue detected" {
		t.Errorf("AgentGoal = %q", s.AgentGoal)
	}
	if s.OnFail != "stop" {
		t.Errorf("OnFail = %q, want stop", s.OnFail)
	}
	if !s.Timeout.Set || s.Timeout.Minutes() != 3 {
		t.Errorf("Timeout = %v", s.Timeout)
	}
	if s.Trap == nil || s.Trap.Name != "delete-policy" {
		t.Errorf("Trap = %v", s.Trap)
	}
	if s.Trap.Points != 10 {
		t.Errorf("Trap.Points = %d, want 10", s.Trap.Points)
	}
	if len(s.Checks) != 1 || s.Checks[0].Type != "deployment-ready" {
		t.Errorf("Checks = %v", s.Checks)
	}
}

func TestScenario_MultiStage_UnmarshalYAML(t *testing.T) {
	t.Parallel()
	input := `
id: multi-test
title: Multi-stage test
category: kubernetes
prompt: prompts/task.md
timeout: 10m
stages:
  - name: stage1
    break:
      type: kubectl-apply
      path: fixtures/break1.yaml
    verify:
      - type: deployment-ready
        namespace: bench
        name: web
  - name: stage2
    break:
      type: kubectl-apply
      path: fixtures/break2.yaml
      memory: reset
    agent_goal: "Fix the secret"
    verify:
      - type: resource-exists
        namespace: bench
        name: db-creds
`
	var s Scenario
	if err := yaml.Unmarshal([]byte(input), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(s.Stages) != 2 {
		t.Fatalf("Stages = %d, want 2", len(s.Stages))
	}
	if s.Stages[0].Name != "stage1" {
		t.Errorf("Stages[0].Name = %q", s.Stages[0].Name)
	}
	if s.Stages[1].Break.Memory != "reset" {
		t.Errorf("Stages[1].Break.Memory = %q, want reset", s.Stages[1].Break.Memory)
	}
	if s.Stages[1].AgentGoal != "Fix the secret" {
		t.Errorf("Stages[1].AgentGoal = %q", s.Stages[1].AgentGoal)
	}
}

func TestScenario_Categories_SingleCategory(t *testing.T) {
	t.Parallel()
	s := Scenario{Category: "terraform"}
	if got := s.PrimaryCategory(); got != "terraform" {
		t.Errorf("PrimaryCategory() = %q, want terraform", got)
	}
	cats := s.ResolvedCategories()
	if len(cats) != 1 || cats[0] != "terraform" {
		t.Errorf("ResolvedCategories() = %v, want [terraform]", cats)
	}
	if !s.HasCategory("terraform") {
		t.Error("HasCategory(terraform) = false, want true")
	}
	if s.HasCategory("aws") {
		t.Error("HasCategory(aws) = true, want false")
	}
}

func TestScenario_Categories_MultiCategory(t *testing.T) {
	t.Parallel()
	s := Scenario{Categories: []string{"terraform", "aws"}}
	if got := s.PrimaryCategory(); got != "terraform" {
		t.Errorf("PrimaryCategory() = %q, want terraform", got)
	}
	cats := s.ResolvedCategories()
	if len(cats) != 2 {
		t.Fatalf("ResolvedCategories() = %v, want [terraform aws]", cats)
	}
	if !s.HasCategory("terraform") {
		t.Error("HasCategory(terraform) = false")
	}
	if !s.HasCategory("aws") {
		t.Error("HasCategory(aws) = false")
	}
	if !s.HasCategory("AWS") {
		t.Error("HasCategory(AWS) = false (case-insensitive)")
	}
	if s.HasCategory("helm") {
		t.Error("HasCategory(helm) = true, want false")
	}
}

func TestScenario_Categories_MultiOverridesSingle(t *testing.T) {
	t.Parallel()
	// When both are set, Categories takes precedence.
	s := Scenario{Category: "old", Categories: []string{"terraform", "aws"}}
	if got := s.PrimaryCategory(); got != "terraform" {
		t.Errorf("PrimaryCategory() = %q, want terraform", got)
	}
	if s.HasCategory("old") {
		t.Error("HasCategory(old) = true; Categories should override Category")
	}
}

func TestScenario_Categories_UnmarshalYAML(t *testing.T) {
	t.Parallel()
	input := `
id: tf-aws-test
title: Terraform AWS test
categories:
  - terraform
  - aws
prompt: prompts/task.md
break:
  type: shell
  path: fixtures/break.sh
checks:
  - type: command-succeeds
    name: verify
    condition: fixtures/verify.sh
`
	var s Scenario
	if err := yaml.Unmarshal([]byte(input), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(s.Categories) != 2 {
		t.Fatalf("Categories = %v, want [terraform aws]", s.Categories)
	}
	if !s.HasCategory("terraform") || !s.HasCategory("aws") {
		t.Errorf("HasCategory failed: %v", s.Categories)
	}
}

func TestScenario_AutopsyHints_UnmarshalYAML(t *testing.T) {
	t.Parallel()
	input := `
id: autopsy-test
title: Autopsy test
category: kubernetes
prompt: prompts/task.md
break:
  type: kubectl-apply
  path: fixtures/broken.yaml
checks:
  - type: deployment-ready
    namespace: bench
    name: web
autopsy:
  expected_diagnostics:
    - kind: command_pattern
      pattern: "kubectl describe deployment"
      reason: "Deployment events reveal image pull failures."
  allowed_mutations:
    - kind: resource_pattern
      pattern: "deployment/*"
  forbidden_actions:
    - kind: command_pattern
      pattern: "kubectl delete namespace"
      severity: critical
  root_cause_resources:
    - deployment/web
`
	var s Scenario
	if err := yaml.Unmarshal([]byte(input), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(s.Autopsy.ExpectedDiagnostics) != 1 {
		t.Fatalf("ExpectedDiagnostics = %v, want one hint", s.Autopsy.ExpectedDiagnostics)
	}
	if got := s.Autopsy.ExpectedDiagnostics[0].Kind; got != "command_pattern" {
		t.Fatalf("expected diagnostic kind = %q, want command_pattern", got)
	}
	if got := s.Autopsy.ExpectedDiagnostics[0].Pattern; got != "kubectl describe deployment" {
		t.Fatalf("expected diagnostic pattern = %q", got)
	}
	if got := s.Autopsy.ExpectedDiagnostics[0].Reason; got == "" {
		t.Fatal("expected diagnostic reason is empty")
	}
	if len(s.Autopsy.AllowedMutations) != 1 || s.Autopsy.AllowedMutations[0].Pattern != "deployment/*" {
		t.Fatalf("AllowedMutations = %#v, want deployment/*", s.Autopsy.AllowedMutations)
	}
	if len(s.Autopsy.ForbiddenActions) != 1 || s.Autopsy.ForbiddenActions[0].Severity != "critical" {
		t.Fatalf("ForbiddenActions = %#v, want critical forbidden action", s.Autopsy.ForbiddenActions)
	}
	if len(s.Autopsy.RootCauseResources) != 1 || s.Autopsy.RootCauseResources[0] != "deployment/web" {
		t.Fatalf("RootCauseResources = %#v, want deployment/web", s.Autopsy.RootCauseResources)
	}
}
