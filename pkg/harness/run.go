// Package harness orchestrates the benchmark run loop.
package harness

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/vitas/evidra-bench/pkg/adapter"
	"github.com/vitas/evidra-bench/pkg/artifact"
	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/environment"
	"github.com/vitas/evidra-bench/pkg/report"
	"github.com/vitas/evidra-bench/pkg/scenario"
	"github.com/vitas/evidra-bench/pkg/store"
	"github.com/vitas/evidra-bench/pkg/verifier"
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
// 7. Optionally report results
func (h *Harness) Run(ctx context.Context, req RunRequest) (result *RunResult, runErr error) {
	startTime := time.Now()
	s := req.Scenario
	ns := targetNamespace(req)
	recorder := newRunArtifactRecorder(startTime)
	var agentResult *adapter.RunResult
	var verifyResult *verifier.VerifyResult
	var promptContent string
	var chaosRun *runChaosHandle

	defer func() {
		if runErr == nil || req.Config.DryRun {
			return
		}
		recorder.Event(recorder.CurrentPhase(), "failed", runErr.Error())
		artifactDir := h.writeFailedRunArtifacts(req, agentResult, verifyResult, promptContent, runChaosRunner(chaosRun), recorder, runErr, startTime, time.Now())
		if result != nil && result.ArtifactDir == "" {
			result.ArtifactDir = artifactDir
		}
	}()

	if req.Config.DryRun {
		log.Printf("[harness] dry-run: skipping environment creation")
		recorder.Event("run", "completed", "dry-run")
		return &RunResult{
			ScenarioID: s.ID,
			Passed:     true,
			Duration:   time.Since(startTime),
		}, nil
	}

	if req.KubeconfigPath == "" {
		recorder.Event("configuration", "failed", "kubeconfig path is required")
		return nil, fmt.Errorf("harness.Run: kubeconfig path is required — caller must acquire a cluster lease")
	}

	// Step 1: Require a pre-acquired cluster lease (kubeconfig path).
	// Cluster creation and destruction are the caller's responsibility.
	handle := &environment.Handle{
		ClusterName:    req.Config.ClusterName,
		KubeconfigPath: req.KubeconfigPath,
	}
	recorder.Event("environment", "started", "")
	cleanupExtraEnv, err := h.prepareRunEnvironment(ctx, req, handle, ns)
	if err != nil {
		recorder.Event("environment", "failed", err.Error())
		return nil, err
	}
	recorder.Event("environment", "completed", "")
	defer cleanupExtraEnv()

	// Step 2d: Bootstrap.
	if h.deps.Bootstrapper != nil {
		recorder.Event("bootstrap", "started", "")
		plan := buildBootstrapPlan(s, req.Config.ScenariosDir)
		if err := h.deps.Bootstrapper.Execute(ctx, plan, handle.KubeconfigPath); err != nil {
			recorder.Event("bootstrap", "failed", err.Error())
			return nil, &InfraError{Err: fmt.Errorf("harness.Run: bootstrap: %w", err)}
		}
		recorder.Event("bootstrap", "completed", "")
	}

	// Step 3: Inject break (skipped for multi-stage — stages handle their own breaks).
	isMultiStage := len(s.Stages) > 0
	if !isMultiStage {
		recorder.Event("break", "started", "")
		if err := h.injectSingleStageBreak(ctx, req, handle.KubeconfigPath); err != nil {
			recorder.Event("break", "failed", err.Error())
			return nil, err
		}
		recorder.Event("break", "completed", "")
	}

	// Step 4: Execute agent.
	recorder.Event("agent_prepare", "started", "")
	promptContent, timeout, err := prepareAgentExecution(req.Config, s)
	if err != nil {
		recorder.Event("agent_prepare", "failed", err.Error())
		return nil, err
	}
	recorder.Event("agent_prepare", "completed", "")
	chaosRun = h.startRunChaos(ctx, s, handle.KubeconfigPath)

	// Step 4: Execute agent (+ concurrent stages for multi-stage).
	recorder.Event("agent_run", "started", "")
	var providerEvDir string
	var stageResults []StageResult
	agentResult, providerEvDir, stageResults, err = h.executeRunAgent(ctx, req, handle.KubeconfigPath, promptContent, timeout, startTime, isMultiStage)
	if err != nil {
		chaosRun.stopForAgentError(s.Chaos)
		recorder.Event("agent_run", "failed", err.Error())
		return nil, wrapRunAgentError(err)
	}
	chaosRun.stopAfterAgentDone(s.Chaos)
	recorder.Event("agent_run", "completed", "")

	// Step 4c: Wait for rollouts to settle before verification.
	recorder.Event("settle", "started", "")
	waitForRollouts(ctx, handle.KubeconfigPath, s)
	recorder.Event("settle", "completed", "")

	// Step 5: Verify outcome.
	recorder.Event("verification", "started", "")
	verifyResult, err = h.verifyRun(ctx, req, handle.KubeconfigPath, agentResult, providerEvDir, stageResults, isMultiStage)
	if err != nil {
		recorder.Event("verification", "failed", err.Error())
		return nil, err
	}
	recorder.Event("verification", "completed", "")

	// Step 6: Write artifacts.
	endTime := time.Now()
	recorder.Event("run", "completed", "")
	recorder.Event("artifact_write", "started", "")
	artifactDir, autopsyJSON := h.writeRunArtifacts(req, agentResult, verifyResult, promptContent, runChaosRunner(chaosRun), recorder, startTime, endTime)
	recorder.Event("artifact_write", "completed", "")

	// Step 7: Bench reporting.
	recorder.Event("report", "started", "")
	h.reportRun(req, agentResult, verifyResult, startTime, endTime)
	recorder.Event("report", "completed", "")

	result = &RunResult{
		ScenarioID:  s.ID,
		Passed:      verifyResult.Passed,
		ExitCode:    agentResult.ExitCode,
		Duration:    endTime.Sub(startTime),
		ArtifactDir: artifactDir,
		Checks:      verifyResult,
	}

	// Step 8: Store result in database.
	recorder.Event("store", "started", "")
	h.storeRun(req, agentResult, verifyResult, artifactDir, autopsyJSON, recorder, startTime, endTime)
	recorder.Event("store", "completed", "")

	return result, nil
}
