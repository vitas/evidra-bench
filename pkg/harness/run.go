// Package harness orchestrates the benchmark run loop.
package harness

import (
	"context"
	"fmt"
	"log"
	"time"

	"samebits.com/evidra-infra-bench/pkg/adapter"
	"samebits.com/evidra-infra-bench/pkg/artifact"
	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/environment"
	"samebits.com/evidra-infra-bench/pkg/report"
	"samebits.com/evidra-infra-bench/pkg/scenario"
	"samebits.com/evidra-infra-bench/pkg/store"
	"samebits.com/evidra-infra-bench/pkg/verifier"
)

// InfraError wraps errors caused by infrastructure problems (cluster degraded,
// node unreachable) as opposed to agent or verification failures.
// Callers can use errors.As to distinguish infra errors from agent failures.
type InfraError struct {
	Err error
}

func (e *InfraError) Error() string { return e.Err.Error() }
func (e *InfraError) Unwrap() error { return e.Err }

// version and commit are set by the CLI at startup via SetVersion.
var (
	version = "dev"
	commit  = "dev"
)

// SetVersion sets the harness version metadata for run artifacts.
func SetVersion(v, c string) {
	version = v
	commit = c
}

// Deps holds all dependencies for the harness.
type Deps struct {
	EnvProvider  environment.ClusterLifecycle
	Bootstrapper *environment.Bootstrapper
	Adapter      adapter.Adapter
	Writer       *artifact.Writer
	Reporter     *report.Reporter
	Store        *store.Store
}

// RunRequest describes what to run.
type RunRequest struct {
	Config          config.Config
	Scenario        *scenario.Scenario
	ExtraEnv        []string // Env vars from the profile lease (e.g., AWS_ENDPOINT_URL from aws-localstack)
	TargetNamespace string   // Override namespace (default: "bench")
	KubeconfigPath  string   // Pre-provisioned kubeconfig — skip cluster create/destroy if set
}

// RunResult holds the outcome of a harness run.
type RunResult struct {
	ScenarioID  string
	Passed      bool
	ExitCode    int
	Duration    time.Duration
	ArtifactDir string
	Checks      *verifier.VerifyResult
}

// Harness orchestrates the benchmark lifecycle.
type Harness struct {
	deps Deps
}

// New creates a Harness with the given dependencies.
func New(deps Deps) *Harness {
	return &Harness{deps: deps}
}

// Run executes the full benchmark lifecycle on a pre-acquired cluster lease:
// 1. Validate kubeconfig (caller must provide a leased cluster)
// 2. Clean namespace + bootstrap baseline
// 3. Inject break
// 4. Execute agent
// 5. Verify outcome
// 6. Write artifacts
// 7. Optionally report to Evidra
func (h *Harness) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	startTime := time.Now()
	s := req.Scenario
	ns := targetNamespace(req)

	if req.Config.DryRun {
		log.Printf("[harness] dry-run: skipping environment creation")
		return &RunResult{
			ScenarioID: s.ID,
			Passed:     true,
			Duration:   time.Since(startTime),
		}, nil
	}

	if req.KubeconfigPath == "" {
		return nil, fmt.Errorf("harness.Run: kubeconfig path is required — caller must acquire a cluster lease")
	}

	// Step 1: Require a pre-acquired cluster lease (kubeconfig path).
	// Cluster creation and destruction are the caller's responsibility.
	handle := &environment.Handle{
		ClusterName:    req.Config.ClusterName,
		KubeconfigPath: req.KubeconfigPath,
	}
	cleanupExtraEnv, err := h.prepareRunEnvironment(ctx, req, handle, ns)
	if err != nil {
		return nil, err
	}
	defer cleanupExtraEnv()

	// Step 2d: Bootstrap.
	if h.deps.Bootstrapper != nil {
		plan := buildBootstrapPlan(s, req.Config.ScenariosDir)
		if err := h.deps.Bootstrapper.Execute(ctx, plan, handle.KubeconfigPath); err != nil {
			return nil, &InfraError{Err: fmt.Errorf("harness.Run: bootstrap: %w", err)}
		}
	}

	// Step 3: Inject break (skipped for multi-stage — stages handle their own breaks).
	isMultiStage := len(s.Stages) > 0
	if !isMultiStage {
		if err := h.injectSingleStageBreak(ctx, req, handle.KubeconfigPath); err != nil {
			return nil, err
		}
	}

	// Step 4: Execute agent.
	promptContent, timeout, err := prepareAgentExecution(req.Config, s)
	if err != nil {
		return nil, err
	}
	chaosRun := h.startRunChaos(ctx, s, handle.KubeconfigPath)

	// Step 4: Execute agent (+ concurrent stages for multi-stage).
	agentResult, providerEvDir, stageResults, err := h.executeRunAgent(ctx, req, handle.KubeconfigPath, promptContent, timeout, startTime, isMultiStage)
	if err != nil {
		chaosRun.stopForAgentError(s.Chaos)
		return nil, wrapRunAgentError(err)
	}
	chaosRun.stopAfterAgentDone(s.Chaos)

	// Step 4c: Wait for rollouts to settle before verification.
	waitForRollouts(ctx, handle.KubeconfigPath, s)

	// Step 5: Verify outcome.
	verifyResult, err := h.verifyRun(ctx, req, handle.KubeconfigPath, agentResult, providerEvDir, stageResults, isMultiStage)
	if err != nil {
		return nil, err
	}

	// Step 6: Write artifacts.
	endTime := time.Now()
	artifactDir, autopsyJSON := h.writeRunArtifacts(req, agentResult, verifyResult, promptContent, runChaosRunner(chaosRun), startTime, endTime)

	// Step 7: Bench reporting.
	h.reportRun(req, agentResult, verifyResult, startTime, endTime)

	result := &RunResult{
		ScenarioID:  s.ID,
		Passed:      verifyResult.Passed,
		ExitCode:    agentResult.ExitCode,
		Duration:    endTime.Sub(startTime),
		ArtifactDir: artifactDir,
		Checks:      verifyResult,
	}

	// Step 8: Store result in database.
	h.storeRun(req, agentResult, verifyResult, artifactDir, autopsyJSON, startTime, endTime)

	return result, nil
}
