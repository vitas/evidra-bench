// Package harness orchestrates the benchmark run loop.
package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"samebits.com/evidra-infra-bench/pkg/adapter"
	"samebits.com/evidra-infra-bench/pkg/agent"
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

	// Resolve target namespace.
	ns := req.TargetNamespace
	if ns == "" {
		ns = config.DefaultNamespace
	}

	// Step 1: Require a pre-acquired cluster lease (kubeconfig path).
	// Cluster creation and destruction are the caller's responsibility.
	var handle *environment.Handle
	var err error
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

	handle = &environment.Handle{
		ClusterName:    req.Config.ClusterName,
		KubeconfigPath: req.KubeconfigPath,
	}

	// Step 1b: Run cloud setup script if the scenario provides one.
	// The profile provisioner (profiles/<profile>/install.sh) already ran and
	// wrote lease.env. ExtraEnv carries the resulting vars (AWS_ENDPOINT_URL,
	// credentials, wrapper PATH) from the lease into the harness process.
	if s.Environment.Cloud.Setup != "" {
		setupCmd := exec.CommandContext(ctx, "bash", s.Environment.Cloud.Setup)
		setupCmd.Env = append(os.Environ(), req.ExtraEnv...)
		if out, setupErr := setupCmd.CombinedOutput(); setupErr != nil {
			return nil, fmt.Errorf("harness.Run: cloud setup: %s: %w", string(out), setupErr)
		}
	}

	// Propagate lease env vars to the process so verifier checks and agent
	// subprocesses can inherit them (e.g. AWS credentials from a profile hook).
	for _, kv := range req.ExtraEnv {
		if parts := strings.SplitN(kv, "=", 2); len(parts) == 2 {
			if err := os.Setenv(parts[0], parts[1]); err != nil {
				return nil, fmt.Errorf("harness.Run: set %s: %w", parts[0], err)
			}
		}
	}
	defer func() {
		for _, kv := range req.ExtraEnv {
			if parts := strings.SplitN(kv, "=", 2); len(parts) == 2 {
				_ = os.Unsetenv(parts[0])
			}
		}
	}()

	// Step 2: Force-clean stale namespace before bootstrap.
	// Namespace deletion is async and can leave finalizers hanging (kubernetes#53327),
	// so we force-delete the namespace entirely and wait for actual removal before recreating.
	kubeconfigExists := handle.KubeconfigPath != ""
	if kubeconfigExists {
		if _, statErr := os.Stat(handle.KubeconfigPath); statErr != nil {
			kubeconfigExists = false
		}
	}
	if kubeconfigExists && h.deps.EnvProvider != nil {
		if err := h.deps.EnvProvider.ForceDeleteNamespace(ctx, handle.KubeconfigPath, ns); err != nil {
			log.Printf("[harness] namespace cleanup %s (non-fatal): %v", ns, err)
		}

		// Also clean cluster-scoped resources that scenarios may create.
		for _, res := range []string{"pv", "storageclass", "validatingwebhookconfiguration", "mutatingwebhookconfiguration"} {
			cleanCmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", handle.KubeconfigPath,
				"delete", res, "--all", "--ignore-not-found", "--timeout=10s")
			if out, err := cleanCmd.CombinedOutput(); err != nil {
				log.Printf("[harness] cluster cleanup %s (non-fatal): %s %v", res, string(out), err)
			}
		}
		// Clean scenario-created namespaces (webhook-system, etc.)
		for _, extraNS := range []string{"webhook-system"} {
			_ = h.deps.EnvProvider.ForceDeleteNamespace(ctx, handle.KubeconfigPath, extraNS)
		}
		// Clean ArgoCD applications (if ArgoCD is installed).
		cleanCmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", handle.KubeconfigPath,
			"delete", "application", "--all", "-n", "argocd", "--ignore-not-found", "--timeout=15s")
		if out, err := cleanCmd.CombinedOutput(); err != nil {
			log.Printf("[harness] argocd application cleanup (non-fatal): %s %v", string(out), err)
		}
	}

	// Step 2b: Recreate target namespace.
	if handle.KubeconfigPath != "" && h.deps.EnvProvider != nil {
		if err := h.deps.EnvProvider.CreateNamespace(ctx, handle.KubeconfigPath, ns); err != nil {
			log.Printf("[harness] namespace create (non-fatal): %v", err)
		}
	}

	// Canary pod — verify scheduling before bootstrap.
	if kubeconfigExists && h.deps.EnvProvider != nil {
		if err := h.deps.EnvProvider.RunCanary(ctx, handle.KubeconfigPath, ns); err != nil {
			return nil, &InfraError{Err: fmt.Errorf("harness.Run: canary failed on leased cluster: %w", err)}
		}
	}

	// Health check — informational after canary.
	if h.deps.EnvProvider != nil {
		if err := h.deps.EnvProvider.HealthCheck(ctx, handle.KubeconfigPath); err != nil {
			log.Printf("[harness] health check warning: %v", err)
		}
	}

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
		if s.Break.Path != "" || s.Break.Type == "kubectl" || s.Break.Type == "shell" {
			if err := h.applyBreak(ctx, handle.KubeconfigPath, s, req.ExtraEnv...); err != nil {
				return nil, fmt.Errorf("harness.Run: inject break: %w", err)
			}
		}
		if h.deps.Bootstrapper != nil && len(s.AfterBreak) > 0 {
			plan := buildStepPlan(s.AfterBreak)
			if err := h.deps.Bootstrapper.Execute(ctx, plan, handle.KubeconfigPath); err != nil {
				return nil, fmt.Errorf("harness.Run: after_break: %w", err)
			}
		}
	}

	// Step 4: Execute agent.
	promptContent := ""
	if s.Prompt != "" {
		data, err := os.ReadFile(s.Prompt)
		if err != nil {
			return nil, fmt.Errorf("harness.Run: read prompt: %w", err)
		}
		promptContent = string(data)
	}

	timeout := req.Config.Timeout
	if s.Timeout.Duration > 0 {
		timeout = s.Timeout.Duration
	}
	if err := os.MkdirAll(req.Config.RunsDir, 0755); err != nil {
		return nil, fmt.Errorf("harness.Run: create runs dir: %w", err)
	}

	var chaosDone chan struct{}
	var chaosCancel context.CancelFunc
	var chaosRunner *ChaosRunner
	if len(s.Chaos.Steps) > 0 {
		chaosCtx, cancel := context.WithCancel(ctx)
		chaosCancel = cancel
		chaosDone = make(chan struct{})
		runner := bootstrapperRunner(h.deps.Bootstrapper)
		chaosRunner = &ChaosRunner{
			Runner:         runner,
			KubeconfigPath: handle.KubeconfigPath,
			Config:         s.Chaos,
		}
		go func() {
			defer close(chaosDone)
			chaosRunner.Run(chaosCtx)
		}()
	}

	// Step 4: Execute agent (+ concurrent stages for multi-stage).
	var agentResult *adapter.RunResult
	var providerEvDir string
	var stageResults []StageResult

	// Create channels for multi-stage agent communication.
	var injectChan chan agent.Message
	var memoryResetChan chan int
	if isMultiStage {
		injectChan = make(chan agent.Message, 4)
		memoryResetChan = make(chan int, 1)
	}

	if isMultiStage && shouldUseProviderEvidenceDir(req.Config) {
		providerEvDir = providerEvidenceDir(req.Config.EvidenceDir, req.Config.RunsDir, s.ID, startTime)

		// Multi-stage: run agent and stages concurrently.
		agentCtx, agentCancel := context.WithCancel(ctx)
		agentDone := make(chan struct{})
		var agentErr error

		go func() {
			defer close(agentDone)
			agentResult, agentErr = h.runWithProvider(agentCtx, req, s, handle.KubeconfigPath, promptContent, timeout, providerEvDir, injectChan, memoryResetChan)
		}()

		// Stage loop runs concurrently — injects breaks, sends goals, polls checks.
		stageErr := h.runMultiStage(ctx, s, handle.KubeconfigPath, injectChan, memoryResetChan, &stageResults)
		// After stages complete, cancel agent context so it finishes.
		agentCancel()
		<-agentDone
		close(injectChan)
		close(memoryResetChan)

		if stageErr != nil {
			return nil, fmt.Errorf("harness.Run: multi-stage: %w", stageErr)
		}
		if agentErr != nil && agentResult == nil {
			return nil, fmt.Errorf("harness.Run: execute agent: %w", agentErr)
		}
		// Agent context cancellation is expected — not an error.
		if agentResult == nil {
			agentResult = &adapter.RunResult{ExitCode: 1}
		}
	} else {
		if shouldUseProviderEvidenceDir(req.Config) {
			providerEvDir = providerEvidenceDir(req.Config.EvidenceDir, req.Config.RunsDir, s.ID, startTime)
		}
		agentResult, err = h.executeSingleAgent(ctx, req, s, handle.KubeconfigPath, promptContent, timeout, providerEvDir)
	}
	if err != nil {
		if chaosCancel != nil && shouldCancelChaosOnAgentDone(s.Chaos) {
			chaosCancel()
		}
		if chaosDone != nil {
			<-chaosDone
		}
		return nil, fmt.Errorf("harness.Run: execute agent: %w", err)
	}
	if chaosCancel != nil && shouldCancelChaosOnAgentDone(s.Chaos) {
		chaosCancel()
	}
	if chaosDone != nil {
		<-chaosDone
		if chaosCancel != nil {
			chaosCancel()
		}
	}

	// Step 4c: Wait for rollouts to settle before verification.
	waitForRollouts(ctx, handle.KubeconfigPath, s)

	// Step 5: Verify outcome.
	var verifyResult *verifier.VerifyResult

	if isMultiStage {
		// Aggregate stage results into a single VerifyResult for artifacts/reporting.
		verifyResult = &verifier.VerifyResult{Passed: true}
		for _, sr := range stageResults {
			if !sr.Passed {
				verifyResult.Passed = false
			}
		}
	} else {
		// Single-stage: existing flow.
		checkDefs := checksToCheckDefs(s.Checks)
		checkers, err := verifier.BuildCheckers(checkDefs)
		if err != nil {
			return nil, fmt.Errorf("harness.Run: build checkers: %w", err)
		}
		verifyResult = verifier.RunChecks(ctx, handle.KubeconfigPath, checkers)
	}
	// Record stage results in metadata for multi-stage runs.
	if len(stageResults) > 0 {
		stagesPassed := 0
		for _, sr := range stageResults {
			if sr.Passed {
				stagesPassed++
			}
		}
		if agentResult.Metadata == nil {
			agentResult.Metadata = make(map[string]string)
		}
		agentResult.Metadata["stages_total"] = strconv.Itoa(len(stageResults))
		agentResult.Metadata["stages_passed"] = strconv.Itoa(stagesPassed)
		stagesJSON, _ := json.Marshal(stageResults)
		agentResult.Metadata["stages"] = string(stagesJSON)
	}

	// Resolve evidence directory for both protocol checks and scorecard.
	evidenceDir := req.Config.EvidenceDir
	if providerEvDir != "" {
		evidenceDir = providerEvDir
	} else if evidenceDir == "" {
		evidenceDir = filepath.Join(req.Config.RunsDir, "evidence")
	}

	// Step 5b: Verify protocol evidence only when a run explicitly supplies an
	// evidence directory. Generic MCP servers are not treated specially here.
	evidenceMode := config.EffectiveEvidenceMode(req.Config)
	if s.Evidra.Enabled && evidenceMode != "none" && req.Config.EvidenceDir != "" {
		// Fall back to simulated evidence if real evidence dir has no segments.
		if s.Evidra.SimulatedEvidenceDir != "" {
			if _, err := os.Stat(filepath.Join(evidenceDir, "segments")); err != nil {
				simDir := s.Evidra.SimulatedEvidenceDir
				if !filepath.IsAbs(simDir) {
					simDir = filepath.Join(s.Dir, simDir)
				}
				evidenceDir = simDir
			}
		}
		evidraCheckers := verifier.BuildEvidraCheckers(verifier.EvidraCheckConfig{
			MinPrescriptions:      s.Evidra.MinPrescriptions,
			MinReports:            s.Evidra.MinReports,
			OrphanedPrescriptions: s.Evidra.OrphanedPrescriptions,
			ProtocolViolations:    s.Evidra.ProtocolViolations,
			AllReportsHaveVerdict: s.Evidra.AllReportsHaveVerdict,
			ExpectedRiskLevel:     s.Evidra.ExpectedRiskLevel,
			ExpectedRiskTags:      s.Evidra.ExpectedRiskTags,
			DeclinedMin:           s.Evidra.DeclinedMin,
			DeclinedMax:           s.Evidra.DeclinedMax,
			RetryLoopMax:          s.Evidra.RetryLoopMax,
			ExpectedSignals:       s.Evidra.ExpectedSignals,
		}, evidenceDir)
		evidraResult := verifier.RunChecks(ctx, handle.KubeconfigPath, evidraCheckers)
		verifyResult.Checks = append(verifyResult.Checks, evidraResult.Checks...)
		if !evidraResult.Passed {
			verifyResult.Passed = false
		}
	}

	// Step 6: Write artifacts.
	endTime := time.Now()
	checksJSON, _ := json.Marshal(verifyResult)
	toolCallsJSON, _ := json.Marshal(agentResult.ToolCalls)
	checksPassedForAutopsy, checksTotalForAutopsy := countChecks(verifyResult)
	autopsyJSON := buildFailureAutopsyJSON(store.RunRecord{
		ScenarioID:       s.ID,
		Model:            req.Config.Model,
		Provider:         req.Config.Provider,
		Adapter:          req.Config.Adapter,
		EvidenceMode:     config.EffectiveEvidenceMode(req.Config),
		Passed:           verifyResult.Passed,
		Duration:         endTime.Sub(startTime).Seconds(),
		ExitCode:         agentResult.ExitCode,
		Turns:            parseIntMeta(agentResult.Metadata, "turns"),
		MemoryWindow:     req.Config.MemoryWindow,
		PromptTokens:     parseIntMeta(agentResult.Metadata, "prompt_tokens"),
		CompletionTokens: parseIntMeta(agentResult.Metadata, "completion_tokens"),
		EstimatedCost:    parseFloatMeta(agentResult.Metadata, "estimated_cost"),
		ChecksPassed:     checksPassedForAutopsy,
		ChecksTotal:      checksTotalForAutopsy,
		ChecksJSON:       string(checksJSON),
		CreatedAt:        startTime,
	}, toolCallsJSON, agentResult.Transcript, checksJSON)
	chaosJSON, chaosLog := chaosArtifacts(chaosRunner)
	chaosStepCount := 0
	chaosMode := ""
	if chaosRunner != nil {
		summary := chaosRunner.Snapshot()
		chaosStepCount = len(summary.Events)
		chaosMode = summary.Mode
	}

	bundle := artifact.RunBundle{
		ScenarioID:     s.ID,
		Adapter:        req.Config.Adapter,
		StartTime:      startTime,
		EndTime:        endTime,
		ExitCode:       agentResult.ExitCode,
		Passed:         verifyResult.Passed,
		Prompt:         promptContent,
		Transcript:     agentResult.Transcript,
		Stdout:         agentResult.Stdout,
		Stderr:         agentResult.Stderr,
		ToolCalls:      toolCallsJSON,
		Checks:         checksJSON,
		Autopsy:        autopsyJSON,
		ChaosEnabled:   chaosRunner != nil,
		ChaosMode:      chaosMode,
		ChaosStepCount: chaosStepCount,
		ChaosTimeline:  chaosJSON,
		ChaosLog:       chaosLog,
		Metadata:       agentResult.Metadata,
	}

	var artifactDir string
	if h.deps.Writer != nil {
		out, err := h.deps.Writer.Write(bundle)
		if err != nil {
			log.Printf("[harness] warning: artifact write failed: %v", err)
		} else {
			artifactDir = out.Path
		}
	}

	// Step 7: Bench reporting.
	if h.deps.Reporter != nil {
		entries := []report.EvidenceEntry{
			{
				ID:         fmt.Sprintf("bench-%s-%d", s.ID, startTime.UnixMilli()),
				Type:       "benchmark-run",
				Actor:      req.Config.Adapter,
				Timestamp:  startTime,
				ScenarioID: s.ID,
				Adapter:    req.Config.Adapter,
				Passed:     verifyResult.Passed,
				ExitCode:   agentResult.ExitCode,
				Duration:   endTime.Sub(startTime),
				Metadata:   agentResult.Metadata,
			},
		}
		if err := h.deps.Reporter.Report(entries); err != nil {
			log.Printf("[harness] warning: evidra report failed: %v", err)
		}
	}

	result := &RunResult{
		ScenarioID:  s.ID,
		Passed:      verifyResult.Passed,
		ExitCode:    agentResult.ExitCode,
		Duration:    endTime.Sub(startTime),
		ArtifactDir: artifactDir,
		Checks:      verifyResult,
	}

	// Step 8: Store result in database.
	if h.deps.Store != nil {
		// Add tool server info to metadata.
		if req.Config.MCPServer != "" {
			agentResult.Metadata["tool_server"] = mcpServerName(req.Config.MCPServer)
			agentResult.Metadata["tool_server_cmd"] = req.Config.MCPServer
			if ver := mcpServerVersion(req.Config.MCPServer); ver != "" {
				agentResult.Metadata["tool_server_version"] = ver
			}
		}
		checksPassed, checksTotal := countChecks(verifyResult)
		checksJSON, _ := json.Marshal(verifyResult)
		metadataJSON, _ := json.Marshal(agentResult.Metadata)

		rec := store.RunRecord{
			ID:               fmt.Sprintf("%s-%s-%s", startTime.Format("20060102-150405"), s.ID, req.Config.Adapter),
			ScenarioID:       s.ID,
			Model:            req.Config.Model,
			Provider:         req.Config.Provider,
			Adapter:          req.Config.Adapter,
			EvidenceMode:     config.EffectiveEvidenceMode(req.Config),
			ToolServer:       mcpServerName(req.Config.MCPServer),
			Passed:           verifyResult.Passed,
			Duration:         endTime.Sub(startTime).Seconds(),
			ExitCode:         agentResult.ExitCode,
			Turns:            parseIntMeta(agentResult.Metadata, "turns"),
			MemoryWindow:     req.Config.MemoryWindow,
			PromptTokens:     parseIntMeta(agentResult.Metadata, "prompt_tokens"),
			CompletionTokens: parseIntMeta(agentResult.Metadata, "completion_tokens"),
			EstimatedCost:    parseFloatMeta(agentResult.Metadata, "estimated_cost"),
			ChecksPassed:     checksPassed,
			ChecksTotal:      checksTotal,
			ChecksJSON:       string(checksJSON),
			MetadataJSON:     string(metadataJSON),
			ArtifactDir:      artifactDir,
			CreatedAt:        startTime,
		}
		if err := h.deps.Store.Insert(rec); err != nil {
			log.Printf("[harness] warning: store insert failed: %v", err)
		}
		// Report to bench API if configured (includes transcript, tool calls, and autopsy).
		ReportToBench(req.Config.BenchURL, req.Config.BenchAPIKey, rec, agentResult.Transcript, agentResult.ToolCalls, autopsyJSON)
	}

	return result, nil
}
