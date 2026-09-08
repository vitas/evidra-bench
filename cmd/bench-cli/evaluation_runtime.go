package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/environment"
	"github.com/vitas/evidra-bench/pkg/evaluation"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

type evaluationRunner interface {
	Run(context.Context, evaluation.Plan) (evaluation.Result, error)
}

type borrowedEvaluationRunner interface {
	RunWithLease(context.Context, evaluation.Plan, evaluation.Lease) (evaluation.Result, error)
}

type reusableEvaluationRunner interface {
	evaluationRunner
	borrowedEvaluationRunner
}

func runLegacySingleEvaluation(ctx context.Context, cfg config.Config, s *scenario.Scenario, runner evaluationRunner) (evaluation.Result, error) {
	return runner.Run(ctx, buildLegacySingleEvaluationPlan(cfg, s))
}

func runLegacySingleEvaluationWithLease(ctx context.Context, cfg config.Config, s *scenario.Scenario, lease evaluation.Lease, runner borrowedEvaluationRunner) (evaluation.Result, error) {
	return runner.RunWithLease(ctx, buildLegacySingleEvaluationPlan(cfg, s), lease)
}

func runLegacyBenchEvaluation(ctx context.Context, cfg config.Config, s *scenario.Scenario, lease *environment.Lease, runner reusableEvaluationRunner) (evaluation.Result, error) {
	if lease == nil {
		return runLegacySingleEvaluation(ctx, cfg, s, runner)
	}
	return runLegacySingleEvaluationWithLease(ctx, cfg, s, &legacyEvaluationLease{lease: lease}, runner)
}

type legacyEvaluationLease struct {
	lease *environment.Lease
}

func (l *legacyEvaluationLease) Release(ctx context.Context) error {
	if l == nil || l.lease == nil {
		return nil
	}
	return l.lease.Release(ctx)
}

type legacyEvaluationProvisioner struct {
	cfg      config.Config
	scenario *scenario.Scenario
}

func (p legacyEvaluationProvisioner) Acquire(ctx context.Context, _ evaluation.Plan) (evaluation.Lease, error) {
	lease, err := newLocalProvisioner(p.cfg).Acquire(ctx, environment.ProvisionRequest{
		Scenario:           p.scenario,
		Profile:            p.scenario.ResolvedProfile(),
		ProviderName:       p.cfg.EnvironmentProvider,
		ClusterName:        p.cfg.ClusterName,
		ReuseCluster:       p.cfg.ReuseCluster,
		ExistingKubeconfig: p.cfg.KubeconfigPath,
	})
	if err != nil {
		return nil, err
	}
	return &legacyEvaluationLease{lease: lease}, nil
}

type legacyEvaluationExecutor struct {
	cfg      config.Config
	scenario *scenario.Scenario
}

func (e legacyEvaluationExecutor) Execute(ctx context.Context, _ evaluation.Plan, c evaluation.CasePlan, lease evaluation.Lease) (evaluation.CaseResult, error) {
	if c.ID != e.scenario.ID {
		return evaluation.CaseResult{}, fmt.Errorf("legacy evaluation: unknown case %q", c.ID)
	}
	localLease, ok := lease.(*legacyEvaluationLease)
	if !ok || localLease.lease == nil {
		return evaluation.CaseResult{}, fmt.Errorf("legacy evaluation: unexpected lease type %T", lease)
	}
	result, err := runScenarioOnceWithLease(ctx, e.cfg, e.scenario, localLease.lease)
	if result != nil && result.Case != nil {
		return *result.Case, err
	}
	if err != nil {
		return evaluation.CaseResult{}, err
	}
	return evaluation.CaseResult{}, fmt.Errorf("legacy evaluation: harness returned no canonical case result")
}

func newLegacySingleEvaluationService(cfg config.Config, s *scenario.Scenario) evaluation.Service {
	return evaluation.Service{
		Provisioner:    legacyEvaluationProvisioner{cfg: cfg, scenario: s},
		Executor:       legacyEvaluationExecutor{cfg: cfg, scenario: s},
		CleanupTimeout: config.GracefulStopTimeout,
	}
}

func buildLegacySingleEvaluationPlan(cfg config.Config, s *scenario.Scenario) evaluation.Plan {
	target := evaluation.TargetPlan{
		Kind:     evaluation.TargetModel,
		Provider: cfg.Provider,
		Model:    cfg.Model,
		Adapter:  cfg.Adapter,
	}
	if cfg.AgentCommand != "" {
		digest := sha256.Sum256([]byte(cfg.AgentCommand))
		target = evaluation.TargetPlan{
			Kind:            evaluation.TargetAgent,
			Adapter:         cfg.Adapter,
			CommandIdentity: "sha256:" + hex.EncodeToString(digest[:]),
		}
	}

	return evaluation.Plan{
		Version: evaluation.PlanVersion,
		Suite: evaluation.SuitePlan{
			ID:     "legacy-single",
			Digest: "unversioned:" + s.ID,
			Cases:  []evaluation.CasePlan{{ID: s.ID}},
		},
		Environment: evaluation.EnvironmentPlan{
			Provider: cfg.EnvironmentProvider,
			Profile:  string(s.ResolvedProfile()),
		},
		Target:   target,
		Limits:   evaluation.Limits{CaseTimeout: cfg.Timeout},
		Attempts: 1,
		Output: evaluation.OutputPolicy{
			Directory: cfg.RunsDir,
			Formats:   []evaluation.OutputFormat{evaluation.OutputTerminal},
		},
	}
}
