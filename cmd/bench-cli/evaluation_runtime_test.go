package main

import (
	"context"
	"testing"
	"time"

	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/evaluation"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

type recordingEvaluationRunner struct {
	plan   evaluation.Plan
	result evaluation.Result
	err    error
}

func (r *recordingEvaluationRunner) Run(_ context.Context, plan evaluation.Plan) (evaluation.Result, error) {
	r.plan = plan
	return r.result, r.err
}

func TestBuildLegacySingleEvaluationPlanNormalizesExistingRunConfiguration(t *testing.T) {
	cfg := config.Default()
	cfg.EnvironmentProvider = "k3d"
	cfg.Provider = "anthropic"
	cfg.Model = "claude-test"
	cfg.Adapter = "cli"
	cfg.Timeout = 90 * time.Second
	cfg.RunsDir = "custom-runs"
	s := &scenario.Scenario{ID: "broken-deployment", Path: "kubernetes/broken-deployment"}

	plan := buildLegacySingleEvaluationPlan(cfg, s)
	if err := plan.Validate(); err != nil {
		t.Fatalf("plan.Validate() error = %v", err)
	}
	if plan.Suite.Cases[0].ID != s.ID || plan.Environment.Provider != "k3d" {
		t.Fatalf("plan suite/environment = %+v/%+v", plan.Suite, plan.Environment)
	}
	if plan.Target.Kind != evaluation.TargetModel || plan.Target.Provider != "anthropic" || plan.Target.Model != "claude-test" {
		t.Fatalf("plan target = %+v", plan.Target)
	}
	if plan.Limits.CaseTimeout != 90*time.Second || plan.Attempts != 1 || plan.Output.Directory != "custom-runs" {
		t.Fatalf("plan limits/output = %+v/%+v", plan.Limits, plan.Output)
	}
}

func TestBuildLegacySingleEvaluationPlanNormalizesExternalAgent(t *testing.T) {
	cfg := config.Default()
	cfg.AgentCommand = "agent --run"
	cfg.Adapter = "cli"
	cfg.Timeout = time.Minute
	s := &scenario.Scenario{ID: "scope", Path: "kubernetes/scope"}

	plan := buildLegacySingleEvaluationPlan(cfg, s)
	if plan.Target.Kind != evaluation.TargetAgent || plan.Target.Adapter != "cli" || plan.Target.CommandIdentity == "" {
		t.Fatalf("plan target = %+v", plan.Target)
	}
	if plan.Target.CommandIdentity == cfg.AgentCommand {
		t.Fatal("plan stores raw external-agent command instead of a safe identity")
	}
}

func TestRunLegacySingleEvaluationDelegatesResolvedPlanToSharedRunner(t *testing.T) {
	cfg := config.Default()
	cfg.Provider = "anthropic"
	cfg.Model = "claude-test"
	cfg.Timeout = time.Minute
	s := &scenario.Scenario{ID: "repair", Path: "kubernetes/repair"}
	runner := &recordingEvaluationRunner{result: evaluation.Result{ID: "eval-1"}}

	result, err := runLegacySingleEvaluation(context.Background(), cfg, s, runner)
	if err != nil {
		t.Fatalf("runLegacySingleEvaluation() error = %v", err)
	}
	if result.ID != "eval-1" || runner.plan.Suite.Cases[0].ID != "repair" {
		t.Fatalf("result/plan = %+v/%+v", result, runner.plan)
	}
}
