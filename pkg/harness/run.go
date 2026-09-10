// Package harness orchestrates the benchmark run loop.
package harness

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/vitas/evidra-bench/pkg/adapter"
	"github.com/vitas/evidra-bench/pkg/agent"
	"github.com/vitas/evidra-bench/pkg/artifact"
	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/environment"
	"github.com/vitas/evidra-bench/pkg/evaluation"
	"github.com/vitas/evidra-bench/pkg/localstore"
	"github.com/vitas/evidra-bench/pkg/report"
	"github.com/vitas/evidra-bench/pkg/scenario"
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
	EnvProvider   environment.ClusterLifecycle
	ModelProvider agent.Provider
	Bootstrapper  *environment.Bootstrapper
	Adapter       adapter.Adapter
	Writer        *artifact.Writer
	Reporter      *report.Reporter
	Store         *localstore.Store
	// Sandbox overrides the docker-CLI sandbox runner (tests); nil uses the
	// production DockerSandbox.
	Sandbox environment.SandboxRunner
}

// RunRequest describes what to run.
type RunRequest struct {
	Config          config.Config
	Scenario        *scenario.Scenario
	ExtraEnv        []string // Env vars from the profile lease (e.g., AWS_ENDPOINT_URL from aws-localstack)
	TargetNamespace string   // Override namespace (default: "bench")
	KubeconfigPath  string   // Pre-provisioned kubeconfig — skip cluster create/destroy if set
	// Audit exposes provisioned API-audit capture on the leased cluster
	// (nil = the cluster has no audit; coverage is then honestly absent).
	Audit *environment.AuditAccess
	// ClusterNetwork is the docker network of the provisioned cluster
	// (agent sandbox attaches there; "" = unknown = sandbox unavailable).
	ClusterNetwork string
}

// RunResult holds the outcome of a harness run.
type RunResult struct {
	ScenarioID  string
	RunID       string
	Passed      bool
	ExitCode    int
	Duration    time.Duration
	ArtifactDir string
	Checks      *verifier.VerifyResult
	Case        *evaluation.CaseResult
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
	runID := buildRunID(startTime, s.ID, req.Config.Model, req.Config.Adapter)
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
		failedAt := time.Now()
		phase := recorder.CurrentPhase()
		artifactDir, safetyAutopsyJSON := h.writeFailedRunArtifacts(req, runID, agentResult, verifyResult, promptContent, runChaosRunner(chaosRun), recorder, runErr, startTime, failedAt)
		kind, _ := classifyRunError(runErr, phase)
		caseResult := buildEvaluationCaseResult(s.ID, runID, agentResult, verifyResult, safetyAutopsyJSON, artifactDir, failedAt.Sub(startTime), evaluation.Termination{
			Kind:    evaluation.TerminationIncomplete,
			Phase:   phase,
			Reason:  kind,
			Details: runErr.Error(),
		}, s.AuthorityProfile != nil, nil, nil, s.AuthorityProfile)
		if result == nil {
			result = &RunResult{
				ScenarioID:  s.ID,
				RunID:       runID,
				Passed:      false,
				ExitCode:    failedRunExitCode(runErr, agentResultExitCode(agentResult)),
				Duration:    time.Since(startTime),
				ArtifactDir: artifactDir,
				Checks:      verifyResult,
				Case:        &caseResult,
			}
		} else {
			if result.RunID == "" {
				result.RunID = runID
			}
			if result.ArtifactDir == "" {
				result.ArtifactDir = artifactDir
			}
			result.Case = &caseResult
		}
	}()

	if req.Config.DryRun {
		log.Printf("[harness] dry-run: skipping environment creation")
		recorder.Event("run", "completed", "dry-run")
		return &RunResult{
			ScenarioID: s.ID,
			RunID:      runID,
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

	// Step 3a: Open the API-audit window (cert-identity start marker).
	auditWin := h.startAuditWindow(ctx, req, handle.KubeconfigPath, recorder)

	// Step 3b: Materialize per-run identities from the authority profile
	// (ADR 0001 Phase 4). With a profile the agent and the verifiers never
	// run on admin credentials: agent = profile grants (SAs below),
	// verifier = evidence-reader. Window markers are emitted by the harness
	// client-certificate identity (admin kubeconfig — spike-proven immune to
	// the bearer cold window). Without a profile nothing is materialized and
	// the run keeps the legacy admin kubeconfig (it is permanently
	// unqualified via gap authority_profile_missing anyway).
	agentKubeconfig := handle.KubeconfigPath
	verifyKubeconfig := handle.KubeconfigPath
	if s.AuthorityProfile != nil {
		recorder.Event("identity", "started", "")
		bundle, err := h.provisionRunIdentities(ctx, req, s, handle.KubeconfigPath, recorder)
		if err != nil {
			return nil, err
		}
		if bundle != nil {
			defer bundle.teardown(ctx)
			agentKubeconfig = bundle.agent.KubeconfigPath
			verifyKubeconfig = bundle.evidence.KubeconfigPath
		}
		recorder.Event("identity", "completed", "")
	}

	// Step 3c: Baseline state checkpoint via the evidence-reader identity
	// (ADR 0001 Phase 7). The agent has not acted yet, so this is the
	// pre-state the preservation diff compares against.
	snaps := newRunSnapshots(s.AuthorityProfile, verifyKubeconfig)
	snaps.capture(ctx, "baseline")

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
	agentResult, providerEvDir, stageResults, err = h.executeRunAgent(ctx, req, agentKubeconfig, promptContent, timeout, startTime, isMultiStage)
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
	verifyResult, err = h.verifyRun(ctx, req, verifyKubeconfig, agentResult, providerEvDir, stageResults, isMultiStage)
	if err != nil {
		recorder.Event("verification", "failed", err.Error())
		return nil, err
	}
	recorder.Event("verification", "completed", "")

	// Step 5a: Post-agent + stability state checkpoints (reader reads are
	// themselves inside the audit window: sealed by the end marker next).
	snaps.capture(ctx, "post-agent")
	time.Sleep(stabilityWindow)
	snaps.capture(ctx, "stability")
	snapInfo := snaps.finalize(recorder)

	// Step 5b: Seal the API-audit window (end marker + drain + redaction).
	auditRes, auditJSONL, auditDigest := auditWin.close(ctx, recorder)
	auditInfo := auditWindowInfo(auditRes, auditJSONL, auditDigest, auditWin)

	// Step 6: Write artifacts.
	endTime := time.Now()
	recorder.Event("run", "completed", "")
	recorder.Event("artifact_write", "started", "")
	artifactDir, autopsyJSON := h.writeRunArtifacts(req, runID, agentResult, verifyResult, promptContent, runChaosRunner(chaosRun), recorder, startTime, endTime, auditInfo, snapInfo)
	recorder.Event("artifact_write", "completed", "")

	// Step 7: Bench reporting.
	recorder.Event("report", "started", "")
	h.reportRun(req, runID, agentResult, verifyResult, startTime, endTime)
	recorder.Event("report", "completed", "")

	result = &RunResult{
		ScenarioID:  s.ID,
		RunID:       runID,
		Passed:      verifyResult.Passed,
		ExitCode:    agentResult.ExitCode,
		Duration:    endTime.Sub(startTime),
		ArtifactDir: artifactDir,
		Checks:      verifyResult,
	}
	caseResult := buildEvaluationCaseResult(s.ID, runID, agentResult, verifyResult, autopsyJSON, artifactDir, endTime.Sub(startTime), evaluation.Termination{Kind: evaluation.TerminationComplete}, s.AuthorityProfile != nil, auditInfo, snapInfo, s.AuthorityProfile)
	result.Case = &caseResult

	// Step 8: Store result in database.
	recorder.Event("store", "started", "")
	h.storeRun(req, runID, agentResult, verifyResult, artifactDir, autopsyJSON, recorder, startTime, endTime)
	recorder.Event("store", "completed", "")

	return result, nil
}
