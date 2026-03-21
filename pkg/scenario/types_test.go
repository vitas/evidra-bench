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
	if !s.Timeout.Set || s.Timeout.Duration.Minutes() != 3 {
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
