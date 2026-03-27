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
	"strings"
	"time"

	"strconv"

	"samebits.com/evidra-infra-bench/pkg/adapter"
	"samebits.com/evidra-infra-bench/pkg/agent"
	"samebits.com/evidra-infra-bench/pkg/artifact"
	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/environment"
	"samebits.com/evidra-infra-bench/pkg/report"
	"samebits.com/evidra-infra-bench/pkg/scenario"
	"samebits.com/evidra-infra-bench/pkg/store"
	"samebits.com/evidra-infra-bench/pkg/verifier"
	"samebits.com/evidra/pkg/proxy"
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

// ClusterHealthChecker probes cluster health and can restart degraded nodes.
type ClusterHealthChecker interface {
	HealthCheck(ctx context.Context, kubeconfigPath string) error
	RestartNode(ctx context.Context, clusterName string) error
}

// ClusterRecreator can destroy and recreate a cluster from scratch.
// Preferred over node restart — avoids IP change issues (kind#2759, kind#3989).
type ClusterRecreator interface {
	RecreateCluster(ctx context.Context, clusterName string) (*environment.Handle, error)
}

// Deps holds all dependencies for the harness.
type Deps struct {
	EnvProvider          environment.Provider
	Bootstrapper         *environment.Bootstrapper
	Runner               environment.CommandRunner
	Adapter              adapter.Adapter
	Writer               *artifact.Writer
	Reporter             *report.Reporter
	Store                *store.Store
	ClusterHealthChecker ClusterHealthChecker
	ClusterRecreator     ClusterRecreator
}

// RunRequest describes what to run.
type RunRequest struct {
	Config          config.Config
	Scenario        *scenario.Scenario
	ExtraEnv        []string // Additional env vars for the agent executor (e.g., AWS_ENDPOINT_URL)
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

// Run executes the full benchmark lifecycle:
// 1. Create environment (or reuse)
// 2. Bootstrap baseline + argocd
// 3. Inject break
// 4. Execute agent
// 5. Verify outcome
// 6. Write artifacts
// 7. Optionally report to Evidra
// 8. Teardown environment
func (h *Harness) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	startTime := time.Now()
	s := req.Scenario

	// Resolve target namespace.
	ns := req.TargetNamespace
	if ns == "" {
		ns = config.DefaultNamespace
	}

	// Step 1: Create or reuse environment.
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

	k8s := s.Environment.Kubernetes

	if req.KubeconfigPath != "" {
		// Pre-provisioned cluster (parallel mode) — skip create/destroy.
		handle = &environment.Handle{
			ClusterName:    req.Config.ClusterName,
			KubeconfigPath: req.KubeconfigPath,
		}
	} else {
		// Use CreateWithConfig when the provider supports it and the scenario
		// declares Kubernetes infrastructure requirements.
		if kp, ok := h.deps.EnvProvider.(*environment.KindProvider); ok && (k8s.CNI != "" || len(k8s.Addons) > 0 || len(k8s.Runtimes) > 0 || len(k8s.Features) > 0) {
			handle, err = kp.CreateWithConfig(ctx, req.Config.ClusterName, k8s)
		} else {
			handle, err = h.deps.EnvProvider.Create(ctx, req.Config.ClusterName)
		}
		if err != nil {
			return nil, fmt.Errorf("harness.Run: create environment: %w", err)
		}
		defer func() {
			if !req.Config.ReuseCluster {
				if destroyErr := h.deps.EnvProvider.Destroy(ctx, handle); destroyErr != nil {
					log.Printf("[harness] warning: destroy failed: %v", destroyErr)
				}
			}
		}()
	}

	// Step 1c: Install addons declared in the scenario's kubernetes config.
	if k8s.CNI != "" || len(k8s.Addons) > 0 {
		addonSteps, warnings := environment.AddonSteps(k8s.CNI, k8s.Addons)
		for _, w := range warnings {
			log.Printf("[harness] warning: %s", w)
		}
		if len(addonSteps) > 0 {
			if err := environment.AddHelmRepos(ctx, h.deps.Runner); err != nil {
				log.Printf("[harness] warning: helm repo setup: %v", err)
			}
			addonPlan := &environment.BootstrapPlan{Steps: addonSteps}
			if err := h.deps.Bootstrapper.Execute(ctx, addonPlan, handle.KubeconfigPath); err != nil {
				return nil, fmt.Errorf("harness.Run: install addons: %w", err)
			}
		}
	}

	// Step 1b: Start LocalStack if scenario requires cloud resources.
	var localstackHandle *environment.LocalStackHandle
	if s.Environment.Cloud.Provider == "localstack" {
		lsHandle, lsErr := environment.StartLocalStack(ctx, req.Config.ClusterName, s.Environment.Cloud.Services)
		if lsErr != nil {
			return nil, fmt.Errorf("harness.Run: start localstack: %w", lsErr)
		}
		localstackHandle = lsHandle
		defer func() {
			if !req.Config.ReuseCluster {
				if err := environment.StopLocalStack(ctx, localstackHandle); err != nil {
					log.Printf("[harness] warning: stop localstack: %v", err)
				}
			}
		}()

		awsEnv := map[string]string{
			"AWS_ENDPOINT_URL":      lsHandle.EndpointURL,
			"AWS_ACCESS_KEY_ID":     "test",
			"AWS_SECRET_ACCESS_KEY": "test",
			"AWS_DEFAULT_REGION":    "us-east-1",
		}

		// Create a wrapper script for `aws` that injects --endpoint-url.
		// This works with all AWS CLI versions (env var requires 2.13+).
		awsBinDir, err := os.MkdirTemp("", "evidra-aws-bin-*")
		if err != nil {
			return nil, fmt.Errorf("harness.Run: create aws wrapper dir: %w", err)
		}
		awsWrapper := filepath.Join(awsBinDir, "aws")
		if err := os.WriteFile(awsWrapper, []byte(fmt.Sprintf(
			"#!/bin/sh\nexec /usr/local/bin/aws --endpoint-url %s \"$@\"\n", lsHandle.EndpointURL,
		)), 0755); err != nil {
			return nil, fmt.Errorf("harness.Run: write aws wrapper: %w", err)
		}
		defer func() { _ = os.RemoveAll(awsBinDir) }()

		// Prepend wrapper dir to PATH so `aws` resolves to our wrapper.
		awsEnv["PATH"] = awsBinDir + ":" + os.Getenv("PATH")

		// Set AWS env vars on the process so verifier checks can inherit them.
		// This is safe: these are test credentials for a local LocalStack container.
		for k, v := range awsEnv {
			if err := os.Setenv(k, v); err != nil {
				return nil, fmt.Errorf("harness.Run: set %s: %w", k, err)
			}
		}
		defer func() {
			for k := range awsEnv {
				_ = os.Unsetenv(k)
			}
		}()

		// Build AWS env vars for subprocess injection (not process-global).
		var awsEnvSlice []string
		for k, v := range awsEnv {
			awsEnvSlice = append(awsEnvSlice, fmt.Sprintf("%s=%s", k, v))
		}
		req.ExtraEnv = append(req.ExtraEnv, awsEnvSlice...)

		// Run cloud setup script if specified.
		if s.Environment.Cloud.Setup != "" {
			setupCmd := exec.CommandContext(ctx, "bash", s.Environment.Cloud.Setup)
			setupCmd.Env = append(os.Environ(), awsEnvSlice...)
			if out, setupErr := setupCmd.CombinedOutput(); setupErr != nil {
				return nil, fmt.Errorf("harness.Run: cloud setup: %s: %w", string(out), setupErr)
			}
		}
	}

	// Step 2: Force-clean stale namespace before bootstrap.
	// Namespace deletion is async and can leave finalizers hanging (kubernetes#53327),
	// so we force-delete the namespace entirely and wait for actual removal before recreating.
	kubeconfigExists := handle.KubeconfigPath != ""
	if kubeconfigExists {
		if _, statErr := os.Stat(handle.KubeconfigPath); statErr != nil {
			kubeconfigExists = false
		}
	}
	if req.Config.ReuseCluster && kubeconfigExists {
		h.forceCleanNamespace(ctx, handle.KubeconfigPath, ns)

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
			h.forceCleanNamespace(ctx, handle.KubeconfigPath, extraNS)
		}
		// Clean ArgoCD applications (if ArgoCD is installed).
		cleanCmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", handle.KubeconfigPath,
			"delete", "application", "--all", "-n", "argocd", "--ignore-not-found", "--timeout=15s")
		if out, err := cleanCmd.CombinedOutput(); err != nil {
			log.Printf("[harness] argocd application cleanup (non-fatal): %s %v", string(out), err)
		}
	}

	// Step 2b: Recreate target namespace.
	if handle.KubeconfigPath != "" {
		createCmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", handle.KubeconfigPath,
			"create", "namespace", ns)
		if out, err := createCmd.CombinedOutput(); err != nil {
			// Namespace may already exist if it wasn't deleted (non-reuse mode).
			if !strings.Contains(string(out), "already exists") {
				log.Printf("[harness] namespace create (non-fatal): %s %v", string(out), err)
			}
		}
	}

	// Step 2c: Canary pod — verify scheduling works before committing to bootstrap.
	if req.Config.ReuseCluster && kubeconfigExists {
		canaryErr := h.runCanaryPod(ctx, handle.KubeconfigPath, ns)
		if canaryErr != nil {
			log.Printf("[harness] canary pod failed: %v", canaryErr)
			// Canary failure means the cluster is too degraded for this scenario.
			// If we have a cluster recreator, try full recreate (safer than node restart,
			// avoids kind IP change issues from kind#2759, kind#3989).
			if h.deps.ClusterRecreator != nil {
				log.Printf("[harness] recreating cluster %s", req.Config.ClusterName)
				newHandle, recreateErr := h.deps.ClusterRecreator.RecreateCluster(ctx, req.Config.ClusterName)
				if recreateErr != nil {
					return nil, &InfraError{Err: fmt.Errorf("harness.Run: canary failed and cluster recreate failed: %w", recreateErr)}
				}
				handle = newHandle
				// Recreate the namespace on the fresh cluster.
				createCmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", handle.KubeconfigPath,
					"create", "namespace", ns)
				if out, err := createCmd.CombinedOutput(); err != nil {
					if !strings.Contains(string(out), "already exists") {
						log.Printf("[harness] namespace create after recreate (non-fatal): %s %v", string(out), err)
					}
				}
				log.Printf("[harness] cluster recreated, continuing with fresh cluster")
			} else {
				return nil, &InfraError{Err: fmt.Errorf("harness.Run: canary pod failed, cluster degraded: %w", canaryErr)}
			}
		}
	}

	// Step 2d: Cluster health probe — check node conditions.
	if h.deps.ClusterHealthChecker != nil {
		if err := h.deps.ClusterHealthChecker.HealthCheck(ctx, handle.KubeconfigPath); err != nil {
			log.Printf("[harness] cluster health check warning: %v", err)
			// Health check is informational after canary — canary is the gate.
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
		runner := h.deps.Runner
		if runner == nil {
			runner = &environment.ExecRunner{}
		}
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

	if req.Config.Provider != "" {
		providerEvDir = providerEvidenceDir(req.Config.EvidraEvidenceDir, req.Config.RunsDir, s.ID, startTime)

		if isMultiStage {
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
			agentResult, err = h.runWithProvider(ctx, req, s, handle.KubeconfigPath, promptContent, timeout, providerEvDir, nil, nil)
		}
	} else {
		agentResult, err = h.deps.Adapter.Run(ctx, adapter.RunInput{
			ScenarioID:     s.ID,
			PromptPath:     s.Prompt,
			WorkspaceDir:   req.Config.RunsDir,
			KubeconfigPath: handle.KubeconfigPath,
			Timeout:        timeout,
			AgentCommand:   req.Config.AgentCommand,
			Model:          req.Config.Model,
		})
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
	evidenceDir := req.Config.EvidraEvidenceDir
	if providerEvDir != "" {
		evidenceDir = providerEvDir
	} else if evidenceDir == "" {
		evidenceDir = filepath.Join(req.Config.RunsDir, "evidence")
	}

	// Step 5b: Verify Evidra protocol compliance.
	// Skip in proxy and smart modes — evidence format differs from evidra's native format.
	evidenceMode := config.EffectiveEvidenceMode(req.Config)
	if s.Evidra.Enabled && evidenceMode != "proxy" && evidenceMode != "smart" && evidenceMode != "mcp" {
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

	// Step 6b: Post-process evidence with evidra scorecard.
	if artifactDir != "" && evidenceDir != "" {
		runEvidraScorecard(req.Config.ResolveEvidraBin(), evidenceDir, artifactDir)
	}

	// Step 7: Evidra reporting.
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
		// Report to evidra API if configured (includes transcript + tool-calls).
		ReportToEvidra(req.Config.EvidraURL, req.Config.EvidraAPIKey, rec, agentResult.Transcript, agentResult.ToolCalls)
	}

	return result, nil
}

func countChecks(vr *verifier.VerifyResult) (passed, total int) {
	if vr == nil {
		return 0, 0
	}
	for _, c := range vr.Checks {
		total++
		if c.Verdict == verifier.VerdictPass {
			passed++
		}
	}
	return
}

func parseIntMeta(meta map[string]string, key string) int {
	v, ok := meta[key]
	if !ok {
		return 0
	}
	n, _ := strconv.Atoi(v)
	return n
}

func parseFloatMeta(meta map[string]string, key string) float64 {
	v, ok := meta[key]
	if !ok {
		return 0
	}
	n, _ := strconv.ParseFloat(v, 64)
	return n
}

// ChaosRunner executes runtime disruption steps while an agent is running.
type ChaosRunner struct {
	Runner         environment.CommandRunner
	KubeconfigPath string
	Config         scenario.ChaosConfig
	events         []chaosEvent
}

// Run executes the configured chaos schedule until completion or cancellation.
func (r *ChaosRunner) Run(ctx context.Context) {
	mode := r.Config.Mode
	if mode == "" {
		mode = "once"
	}
	for {
		cycleStart := time.Now()
		for _, step := range r.Config.Steps {
			scheduledAt := cycleStart.Add(step.At.Duration)
			if err := waitForChaosStep(ctx, cycleStart, step.At.Duration); err != nil {
				return
			}
			event := r.executeStep(ctx, step, scheduledAt)
			r.events = append(r.events, event)
			if event.Error != "" {
				if step.AllowFailure {
					log.Printf("[chaos] step %s failed as allowed: %s", step.Name, event.Error)
					continue
				}
				log.Printf("[chaos] step %s failed: %s", step.Name, event.Error)
			}
		}
		if mode != "repeat" {
			return
		}
	}
}

func (r *ChaosRunner) executeStep(ctx context.Context, step scenario.ChaosStep, scheduledAt time.Time) chaosEvent {
	event := chaosEvent{
		Name:         step.Name,
		Type:         step.Type,
		ScheduledAt:  scheduledAt,
		AllowFailure: step.AllowFailure,
		Command:      chaosCommandArgs(r.KubeconfigPath, step),
	}
	event.StartedAt = time.Now()
	if step.Type == "sleep" {
		duration, err := time.ParseDuration(step.Duration)
		if err != nil {
			event.FinishedAt = time.Now()
			event.Error = fmt.Sprintf("parse chaos sleep duration %q: %v", step.Duration, err)
			return event
		}
		if err := sleepContext(ctx, duration); err != nil {
			event.FinishedAt = time.Now()
			event.Error = err.Error()
			return event
		}
		event.FinishedAt = time.Now()
		event.Success = true
		return event
	}

	envStep := environment.BootstrapStep{
		Name:      step.Name,
		Type:      environment.StepType(step.Type),
		Path:      step.Path,
		Release:   step.Release,
		Namespace: step.Namespace,
		Duration:  step.Duration,
		Args:      append([]string(nil), step.Args...),
	}
	args := envStep.CommandArgs(r.KubeconfigPath)
	if len(args) == 0 {
		event.FinishedAt = time.Now()
		event.Error = fmt.Sprintf("no command for chaos step %s", step.Name)
		return event
	}
	cmd := makeCmd(args)
	out, err := r.Runner.Run(ctx, cmd)
	event.FinishedAt = time.Now()
	event.Output = strings.TrimSpace(string(out))
	if err != nil {
		event.Error = err.Error()
		return event
	}
	event.Success = true
	return event
}

func waitForChaosStep(ctx context.Context, cycleStart time.Time, at time.Duration) error {
	wait := at - time.Since(cycleStart)
	if wait <= 0 {
		return nil
	}
	return sleepContext(ctx, wait)
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func shouldCancelChaosOnAgentDone(cfg scenario.ChaosConfig) bool {
	return cfg.StopOnAgentDone || cfg.Mode == "repeat"
}

func chaosCommandArgs(kubeconfigPath string, step scenario.ChaosStep) []string {
	if step.Type == "sleep" {
		return nil
	}
	envStep := environment.BootstrapStep{
		Name:      step.Name,
		Type:      environment.StepType(step.Type),
		Path:      step.Path,
		Release:   step.Release,
		Namespace: step.Namespace,
		Duration:  step.Duration,
		Args:      append([]string(nil), step.Args...),
	}
	return envStep.CommandArgs(kubeconfigPath)
}

func chaosArtifacts(r *ChaosRunner) (json.RawMessage, string) {
	if r == nil {
		return nil, ""
	}
	summary := r.Snapshot()
	if len(summary.Events) == 0 {
		return nil, ""
	}
	data, err := json.Marshal(summary)
	if err != nil {
		return nil, ""
	}
	return data, summary.Log()
}

// Snapshot returns the executed chaos timeline.
func (r *ChaosRunner) Snapshot() chaosSummary {
	mode := r.Config.Mode
	if mode == "" {
		mode = "once"
	}
	events := append([]chaosEvent(nil), r.events...)
	return chaosSummary{
		Mode:            mode,
		StopOnAgentDone: shouldCancelChaosOnAgentDone(r.Config),
		Events:          events,
	}
}

type chaosSummary struct {
	Mode            string       `json:"mode"`
	StopOnAgentDone bool         `json:"stop_on_agent_done"`
	Events          []chaosEvent `json:"events"`
}

type chaosEvent struct {
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	ScheduledAt  time.Time `json:"scheduled_at"`
	StartedAt    time.Time `json:"started_at"`
	FinishedAt   time.Time `json:"finished_at"`
	Command      []string  `json:"command,omitempty"`
	Success      bool      `json:"success"`
	AllowFailure bool      `json:"allow_failure,omitempty"`
	Output       string    `json:"output,omitempty"`
	Error        string    `json:"error,omitempty"`
}

func (s chaosSummary) Log() string {
	var b strings.Builder
	for _, event := range s.Events {
		status := "ok"
		if event.Error != "" {
			status = "error: " + event.Error
		}
		fmt.Fprintf(&b,
			"%s %s %s %s\n",
			event.StartedAt.Format(time.RFC3339Nano),
			event.Name,
			event.Type,
			status,
		)
	}
	return b.String()
}

func (h *Harness) applyBreak(ctx context.Context, kubeconfigPath string, s *scenario.Scenario, extraEnv ...string) error {
	runner := h.deps.Runner
	if runner == nil {
		runner = &environment.ExecRunner{}
	}
	args, err := breakCommandArgs(kubeconfigPath, s)
	if err != nil {
		return err
	}
	cmd := makeCmd(args)
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	if _, err := runner.Run(ctx, cmd); err != nil {
		if s.Break.AllowFailure {
			log.Printf("[harness] break command failed as expected for scenario %s: %v", s.ID, err)
			return nil
		}
		return fmt.Errorf("apply break fixture: %w", err)
	}
	return nil
}

func buildBootstrapPlan(s *scenario.Scenario, scenariosDir string) *environment.BootstrapPlan {
	plan := environment.DefaultBootstrapPlan()
	rootDir := filepath.Dir(scenariosDir)
	for i := range plan.Steps {
		if plan.Steps[i].Path != "" && rootDir != "" && !filepath.IsAbs(plan.Steps[i].Path) {
			plan.Steps[i].Path = filepath.Join(rootDir, plan.Steps[i].Path)
		}
	}
	plan.Steps = append(plan.Steps, buildStepPlan(s.Bootstrap).Steps...)
	return plan
}

func buildStepPlan(steps []scenario.BootstrapStep) *environment.BootstrapPlan {
	plan := &environment.BootstrapPlan{}
	for _, step := range steps {
		plan.Steps = append(plan.Steps, environment.BootstrapStep{
			Name:      step.Name,
			Type:      environment.StepType(step.Type),
			Path:      step.Path,
			Release:   step.Release,
			Namespace: step.Namespace,
			Duration:  step.Duration,
			Args:      append([]string(nil), step.Args...),
		})
	}
	return plan
}

func (h *Harness) runWithProvider(ctx context.Context, req RunRequest, s *scenario.Scenario, kubeconfigPath, promptContent string, timeout time.Duration, evidenceDir string, injectChan <-chan agent.Message, memoryResetChan <-chan int) (*adapter.RunResult, error) {
	cfg := req.Config
	if config.IsSupportedEvidenceMode(cfg.EvidenceMode) {
		cfg = config.ApplyEvidenceMode(cfg, cfg.EvidenceMode)
	}

	provider, err := agent.ResolveProvider(cfg.Provider)
	if err != nil {
		return nil, err
	}

	if evidenceDir == "" {
		evidenceDir = providerEvidenceDir(cfg.EvidraEvidenceDir, cfg.RunsDir, s.ID, time.Now())
	}
	if err := os.MkdirAll(evidenceDir, 0755); err != nil {
		return nil, fmt.Errorf("harness: create evidence dir: %w", err)
	}

	systemPrompt, err := buildSystemPrompt(cfg, s)
	if err != nil {
		return nil, err
	}

	agentCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Choose executor: MCP server or direct.
	var loopExecutor agent.Executor
	var mcpTools []agent.ToolDef
	var mcpExec *agent.MCPExecutor

	if cfg.MCPServer != "" {
		// MCP mode: route tool calls through MCP server.
		mcpEnv := append(req.ExtraEnv, "KUBECONFIG="+kubeconfigPath)
		if evidenceDir != "" {
			mcpEnv = append(mcpEnv, "EVIDRA_EVIDENCE_DIR="+evidenceDir)
		}
		var mcpErr error
		mcpExec, mcpErr = agent.NewMCPExecutor(agentCtx, cfg.MCPServer, mcpEnv)
		if mcpErr != nil {
			return nil, fmt.Errorf("harness: mcp executor: %w", mcpErr)
		}
		defer func() { _ = mcpExec.Close() }()
		loopExecutor = mcpExec

		// Get tools from MCP server.
		var toolErr error
		mcpTools, toolErr = mcpExec.Tools(agentCtx)
		if toolErr != nil {
			return nil, fmt.Errorf("harness: mcp tools: %w", toolErr)
		}
		log.Printf("[harness] using MCP server: %s (%d tools)", cfg.MCPServer, len(mcpTools))
	} else {
		// Direct mode: harness executes commands.
		executor := &agent.ToolExecutor{
			KubeconfigPath: kubeconfigPath,
			EvidencePath:   evidenceDir,
			EvidraBin:      cfg.ResolveEvidraBin(),
			ExtraEnv:       req.ExtraEnv,
		}
		loopExecutor = executor

		if cfg.SmartPrescribe {
			// Smart mode: simplified prescribe schema, no evidra binary needed.
			evidence, evErr := proxy.NewEvidenceWriter(evidenceDir)
			if evErr != nil {
				return nil, fmt.Errorf("harness: smart evidence: %w", evErr)
			}
			defer func() { _ = evidence.Close() }()
			loopExecutor = &agent.SmartToolExecutor{Base: executor, Evidence: evidence}
		} else if cfg.ProxyMode {
			// Proxy mode: auto-record, agent unaware.
			proxyEvidence, proxyErr := proxy.NewEvidenceWriter(evidenceDir)
			if proxyErr != nil {
				return nil, fmt.Errorf("harness: proxy evidence: %w", proxyErr)
			}
			defer func() { _ = proxyEvidence.Close() }()
			executor.ProxyEvidence = proxyEvidence
		}
	}

	loopResult, err := agent.RunLoop(agentCtx, agent.LoopConfig{
		Provider:        provider,
		Executor:        loopExecutor,
		Model:           cfg.Model,
		MaxTurns:        25,
		MemoryWindow:    cfg.MemoryWindow,
		SystemPrompt:    systemPrompt,
		TaskPrompt:      promptContent,
		Tools:           mcpTools,
		InjectChan:      injectChan,
		MemoryResetChan: memoryResetChan,
	})
	if err != nil {
		return nil, fmt.Errorf("harness: agent loop: %w", err)
	}

	// Build transcript from messages
	var transcript strings.Builder
	for _, m := range loopResult.Messages {
		fmt.Fprintf(&transcript, "[%s] %s\n", m.Role, truncateForLog(m.Content, 500))
		for _, tc := range m.ToolCalls {
			fmt.Fprintf(&transcript, "  -> %s(%s)\n", tc.Name, truncateForLog(tc.Arguments, 200))
		}
	}

	exitCode := 0
	if !loopResult.Completed {
		exitCode = 1
	}

	return &adapter.RunResult{
		ExitCode:   exitCode,
		Transcript: transcript.String(),
		Stdout:     loopResult.FinalOutput,
		ToolCalls:  providerToolCalls(loopResult.Messages),
		Metadata:   buildRunMetadata(cfg, loopResult, evidenceDir),
	}, nil
}

// buildRunMetadata creates the metadata map for a provider-path run,
// including all version information for reproducibility.
func buildRunMetadata(cfg config.Config, loopResult *agent.LoopResult, evidenceDir string) map[string]string {
	if config.IsSupportedEvidenceMode(cfg.EvidenceMode) {
		cfg = config.ApplyEvidenceMode(cfg, cfg.EvidenceMode)
	}

	meta := map[string]string{
		"provider":          cfg.Provider,
		"model":             cfg.Model,
		"evidence_mode":     config.EffectiveEvidenceMode(cfg),
		"turns":             fmt.Sprintf("%d", loopResult.Turns),
		"memory_window":     fmt.Sprintf("%d", loopResult.MemoryWindow),
		"prompt_tokens":     fmt.Sprintf("%d", loopResult.TotalUsage.PromptTokens),
		"completion_tokens": fmt.Sprintf("%d", loopResult.TotalUsage.CompletionTokens),
		"estimated_cost":    agent.EstimateCost(cfg.Model, loopResult.TotalUsage).String(),
		"evidence_dir":      evidenceDir,
	}
	// Merge version info
	vi := config.CollectVersions(version, commit, cfg)
	for k, v := range vi.ToMetadata() {
		meta[k] = v
	}
	return meta
}

// buildSystemPrompt loads the system prompt from file, role skill, or returns the default.
func buildSystemPrompt(cfg config.Config, s *scenario.Scenario) (string, error) {
	// 1. Explicit system prompt file takes precedence over everything.
	promptFile := cfg.ResolveSystemPromptFile()
	if promptFile != "" {
		data, err := os.ReadFile(promptFile)
		if err != nil {
			return "", fmt.Errorf("harness: read system prompt file: %w", err)
		}
		prompt := string(data)
		prompt += fmt.Sprintf("\n\nTarget namespace: %s\n", strings.Join(s.Scope.Namespaces, ", "))
		return prompt, nil
	}

	// 2. Role-based skill: load from skills/<role>.md
	if cfg.Role != "" {
		skillPath := filepath.Join(cfg.ScenariosDir, "..", "skills", cfg.Role+".md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			return "", fmt.Errorf("harness: role skill %q not found at %s: %w", cfg.Role, skillPath, err)
		}
		prompt := string(data)
		prompt += fmt.Sprintf("\n\nTarget namespace: %s\n", strings.Join(s.Scope.Namespaces, ", "))
		log.Printf("[harness] loaded role skill: %s (%d bytes)", cfg.Role, len(data))
		return prompt, nil
	}

	// 3. Smart prescribe mode: auto-load the smart prescribe skill if no prompt file given.
	if cfg.SmartPrescribe {
		// Load smart prescribe skill. Source of truth: evidra/prompts/skill/SKILL_SMART.md
		skillPath := filepath.Join(cfg.ScenariosDir, "..", "skills", "evidra", "smart-prescribe.md")
		if data, err := os.ReadFile(skillPath); err == nil {
			prompt := string(data)
			prompt += fmt.Sprintf("\n\nTarget namespace: %s\n", strings.Join(s.Scope.Namespaces, ", "))
			return prompt, nil
		}
		// Fallback: inline minimal smart prescribe instructions.
		return fmt.Sprintf(
			"You are an infrastructure agent. Fix the problem described in the task.\n"+
				"KUBECONFIG is already set. Use kubectl, helm, or other tools via the run_command tool.\n"+
				"For read-only commands (get, describe, logs): just use run_command directly.\n\n"+
				"IMPORTANT: Before every infrastructure mutation (kubectl apply/patch/delete, helm upgrade, etc.),\n"+
				"call evidra_prescribe_smart with tool, operation, resource, namespace, and actor fields.\n"+
				"After the mutation, call evidra_report with the prescription_id and verdict.\n"+
				"Skip the protocol for read-only commands.\n\n"+
				"Namespace: %s",
			strings.Join(s.Scope.Namespaces, ", "),
		), nil
	}

	// Default prompt — no protocol skill
	return fmt.Sprintf(
		"You are an infrastructure agent. Fix the problem described in the task.\n"+
			"KUBECONFIG is already set. Use kubectl, helm, or other tools via the run_command tool.\n"+
			"For read-only commands (get, describe, logs): just use run_command directly.\n"+
			"Namespace: %s",
		strings.Join(s.Scope.Namespaces, ", "),
	), nil
}

func providerEvidenceDir(configuredRoot, runsDir, scenarioID string, started time.Time) string {
	root := configuredRoot
	if root == "" {
		root = filepath.Join(runsDir, "evidence")
	}
	safeScenarioID := strings.ReplaceAll(scenarioID, "/", "-")
	return filepath.Join(root, fmt.Sprintf("%s-%d", safeScenarioID, started.UnixMilli()))
}

func providerToolCalls(messages []agent.Message) []adapter.ToolCall {
	var calls []adapter.ToolCall
	callIndexByID := map[string]int{}

	for _, msg := range messages {
		switch msg.Role {
		case "assistant":
			for _, tc := range msg.ToolCalls {
				args := map[string]any{}
				if strings.TrimSpace(tc.Arguments) != "" {
					_ = json.Unmarshal([]byte(tc.Arguments), &args)
				}
				calls = append(calls, adapter.ToolCall{
					Tool: tc.Name,
					Args: args,
				})
				callIndexByID[tc.ID] = len(calls) - 1
			}
		case "tool":
			if idx, ok := callIndexByID[msg.ToolCallID]; ok {
				calls[idx].Result = msg.Content
			}
		}
	}

	return calls
}

// waitForRollouts gives deployments time to converge after the agent finishes.
func waitForRollouts(ctx context.Context, kubeconfigPath string, s *scenario.Scenario) {
	for _, check := range s.Checks {
		if check.Type != "deployment-ready" {
			continue
		}
		ns := check.Namespace
		if ns == "" {
			ns = config.DefaultNamespace
		}
		waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		cmd := exec.CommandContext(waitCtx, "kubectl",
			"--kubeconfig", kubeconfigPath,
			"rollout", "status",
			fmt.Sprintf("deployment/%s", check.Name),
			"-n", ns,
			"--timeout=30s",
		)
		out, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			log.Printf("[harness] rollout wait for %s/%s: %v: %s", ns, check.Name, err, strings.TrimSpace(string(out)))
		}
	}
}

// runEvidraScorecard runs `evidra scorecard` on the evidence and saves
// scorecard.json into the artifact directory for audit consumption.
func runEvidraScorecard(evidraBin, evidenceDir, artifactDir string) {
	if evidraBin == "" {
		return
	}
	// Find the actual evidence session dir (may be a subdirectory)
	var sessionDir string
	_ = filepath.WalkDir(evidenceDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() || d.Name() != "segments" {
			return nil
		}
		sessionDir = filepath.Dir(path)
		return filepath.SkipAll
	})
	if sessionDir == "" {
		return
	}

	cmd := exec.CommandContext(context.Background(), evidraBin, "scorecard", "--evidence-dir", sessionDir, "--ttl", "1s")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[harness] scorecard failed: %v", err)
		return
	}

	scorecardPath := filepath.Join(artifactDir, "scorecard.json")
	if writeErr := os.WriteFile(scorecardPath, out, 0644); writeErr != nil {
		log.Printf("[harness] warning: write scorecard.json: %v", writeErr)
	} else {
		log.Printf("[harness] scorecard written: %s", scorecardPath)
	}
}

// mcpServerName extracts the binary name from a full MCP server command.
// "evidra-mcp --signing-mode optional" → "evidra-mcp"
func mcpServerName(cmd string) string {
	if cmd == "" {
		return ""
	}
	parts := strings.Fields(cmd)
	return parts[0]
}

// mcpServerVersion queries the MCP server binary for its version string.
func mcpServerVersion(cmd string) string {
	if cmd == "" {
		return ""
	}
	parts := strings.Fields(cmd)
	out, err := exec.Command(parts[0], "--version").CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func truncateForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func breakCommandArgs(kubeconfigPath string, s *scenario.Scenario) ([]string, error) {
	switch s.Break.Type {
	case "", "apply", "kubectl-apply":
		if s.Break.Path == "" {
			return nil, fmt.Errorf("break fixture path is required for %q", s.Break.Type)
		}
		return []string{"kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", s.Break.Path}, nil
	case "kubectl":
		args := []string{"kubectl", "--kubeconfig", kubeconfigPath}
		return append(args, s.Break.Args...), nil
	case "shell":
		args := []string{"bash", s.Break.Path, kubeconfigPath}
		return append(args, s.Break.Args...), nil
	case "helm-upgrade":
		if s.Break.Name == "" {
			return nil, fmt.Errorf("break release name is required for helm-upgrade")
		}
		if s.Break.Chart == "" {
			return nil, fmt.Errorf("break chart is required for helm-upgrade")
		}
		args := []string{"helm", "--kubeconfig", kubeconfigPath, "upgrade", s.Break.Name, s.Break.Chart}
		if s.Break.Namespace != "" {
			args = append(args, "-n", s.Break.Namespace)
		}
		if s.Break.Path != "" {
			args = append(args, "-f", s.Break.Path)
		}
		return append(args, s.Break.Args...), nil
	default:
		return nil, fmt.Errorf("unsupported break type: %s", s.Break.Type)
	}
}

// runCanaryPod runs a lightweight pod to verify the cluster can schedule and pull images.
// Uses busybox (typically cached on kind nodes) as a basic scheduling test.
func (h *Harness) runCanaryPod(ctx context.Context, kubeconfigPath, ns string) error {
	// Clean up any leftover canary pod from a prior failed attempt.
	delCmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath,
		"delete", "pod", "bench-canary", "-n", ns, "--ignore-not-found", "--timeout=10s")
	_, _ = delCmd.CombinedOutput()

	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath,
		"run", "bench-canary", "-n", ns,
		"--image=busybox:1.36", "--restart=Never",
		"--rm", "-i", "--timeout=30s", "--", "echo", "ok")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("canary pod failed: %w: %s", err, string(out))
	}
	return nil
}

// forceCleanNamespace force-deletes a namespace and waits for it to be fully removed.
// This avoids the async deletion + finalizer accumulation problem (kubernetes#53327).
func (h *Harness) forceCleanNamespace(ctx context.Context, kubeconfigPath, ns string) {
	// Check if namespace exists.
	getCmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath,
		"get", "namespace", ns, "--ignore-not-found", "-o", "name")
	getOut, err := getCmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(getOut)) == "" {
		return // Namespace doesn't exist, nothing to clean.
	}

	log.Printf("[harness] force-deleting namespace %s", ns)

	// Force-delete the namespace.
	delCmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath,
		"delete", "namespace", ns, "--force", "--grace-period=0", "--timeout=30s")
	if out, err := delCmd.CombinedOutput(); err != nil {
		log.Printf("[harness] namespace force-delete %s (non-fatal): %s %v", ns, string(out), err)
	}

	// Wait for the namespace to actually disappear (up to 30s).
	for i := 0; i < 30; i++ {
		checkCmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath,
			"get", "namespace", ns, "--ignore-not-found", "-o", "name")
		checkOut, _ := checkCmd.CombinedOutput()
		if strings.TrimSpace(string(checkOut)) == "" {
			log.Printf("[harness] namespace %s deleted", ns)
			return
		}
		time.Sleep(1 * time.Second)
	}
	log.Printf("[harness] warning: namespace %s still terminating after 30s", ns)
}
