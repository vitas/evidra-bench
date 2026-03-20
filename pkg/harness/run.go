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
)

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
	EnvProvider  environment.Provider
	Bootstrapper *environment.Bootstrapper
	Runner       environment.CommandRunner
	Adapter      adapter.Adapter
	Writer       *artifact.Writer
	Reporter     *report.Reporter
	Store        *store.Store
}

// RunRequest describes what to run.
type RunRequest struct {
	Config   config.Config
	Scenario *scenario.Scenario
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

	handle, err = h.deps.EnvProvider.Create(ctx, req.Config.ClusterName)
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

	// Step 2: Bootstrap.
	if h.deps.Bootstrapper != nil {
		plan := buildBootstrapPlan(s, req.Config.ScenariosDir)
		if err := h.deps.Bootstrapper.Execute(ctx, plan, handle.KubeconfigPath); err != nil {
			return nil, fmt.Errorf("harness.Run: bootstrap: %w", err)
		}
	}

	// Step 3: Inject break.
	if s.Break.Path != "" || s.Break.Type == "kubectl" || s.Break.Type == "shell" {
		if err := h.applyBreak(ctx, handle.KubeconfigPath, s); err != nil {
			return nil, fmt.Errorf("harness.Run: inject break: %w", err)
		}
	}
	if h.deps.Bootstrapper != nil && len(s.AfterBreak) > 0 {
		plan := buildStepPlan(s.AfterBreak)
		if err := h.deps.Bootstrapper.Execute(ctx, plan, handle.KubeconfigPath); err != nil {
			return nil, fmt.Errorf("harness.Run: after_break: %w", err)
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

	var agentResult *adapter.RunResult
	var providerEvDir string
	if req.Config.Provider != "" {
		providerEvDir = providerEvidenceDir(req.Config.EvidraEvidenceDir, req.Config.RunsDir, s.ID, startTime)
		agentResult, err = h.runWithProvider(ctx, req, s, handle.KubeconfigPath, promptContent, timeout, providerEvDir)
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
	var checkDefs []verifier.CheckDef
	for _, c := range s.Checks {
		checkDefs = append(checkDefs, verifier.CheckDef{
			Type:      c.Type,
			Namespace: c.Namespace,
			Name:      c.Name,
			Condition: c.Condition,
		})
	}
	checkers, err := verifier.BuildCheckers(checkDefs)
	if err != nil {
		return nil, fmt.Errorf("harness.Run: build checkers: %w", err)
	}
	verifyResult := verifier.RunChecks(ctx, handle.KubeconfigPath, checkers)

	// Resolve evidence directory for both protocol checks and scorecard.
	evidenceDir := req.Config.EvidraEvidenceDir
	if providerEvDir != "" {
		evidenceDir = providerEvDir
	} else if evidenceDir == "" {
		evidenceDir = filepath.Join(req.Config.RunsDir, "evidence")
	}

	// Step 5b: Verify Evidra protocol compliance.
	// Skip in proxy and smart modes — evidence format differs from evidra's native format.
	if s.Evidra.Enabled && !req.Config.ProxyMode && !req.Config.SmartPrescribe {
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
		checksPassed, checksTotal := countChecks(verifyResult)
		checksJSON, _ := json.Marshal(verifyResult)
		metadataJSON, _ := json.Marshal(agentResult.Metadata)
		// Determine evidence mode for the record.
		evidenceMode := "none"
		if req.Config.SmartPrescribe {
			evidenceMode = "smart"
		} else if req.Config.ProxyMode {
			evidenceMode = "proxy"
		} else if req.Config.ResolveEvidraBin() != "" {
			evidenceMode = "direct"
		}

		rec := store.RunRecord{
			ID:               fmt.Sprintf("%s-%s-%s", startTime.Format("20060102-150405"), s.ID, req.Config.Adapter),
			ScenarioID:       s.ID,
			Model:            req.Config.Model,
			Provider:         req.Config.Provider,
			Adapter:          req.Config.Adapter,
			EvidenceMode:     evidenceMode,
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
		b.WriteString(fmt.Sprintf(
			"%s %s %s %s\n",
			event.StartedAt.Format(time.RFC3339Nano),
			event.Name,
			event.Type,
			status,
		))
	}
	return b.String()
}

func (h *Harness) applyBreak(ctx context.Context, kubeconfigPath string, s *scenario.Scenario) error {
	runner := h.deps.Runner
	if runner == nil {
		runner = &environment.ExecRunner{}
	}
	args, err := breakCommandArgs(kubeconfigPath, s)
	if err != nil {
		return err
	}
	cmd := makeCmd(args)
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

func (h *Harness) runWithProvider(ctx context.Context, req RunRequest, s *scenario.Scenario, kubeconfigPath, promptContent string, timeout time.Duration, evidenceDir string) (*adapter.RunResult, error) {
	provider, err := agent.ResolveProvider(req.Config.Provider)
	if err != nil {
		return nil, err
	}

	if evidenceDir == "" {
		evidenceDir = providerEvidenceDir(req.Config.EvidraEvidenceDir, req.Config.RunsDir, s.ID, time.Now())
	}
	if err := os.MkdirAll(evidenceDir, 0755); err != nil {
		return nil, fmt.Errorf("harness: create evidence dir: %w", err)
	}

	systemPrompt, err := buildSystemPrompt(req.Config, s)
	if err != nil {
		return nil, err
	}

	agentCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	executor := &agent.ToolExecutor{
		KubeconfigPath: kubeconfigPath,
		EvidencePath:   evidenceDir,
		EvidraBin:      req.Config.ResolveEvidraBin(),
	}

	// Evidence mode determines which executor and tools the agent gets.
	var loopExecutor agent.Executor = executor

	if req.Config.SmartPrescribe {
		// Smart mode: simplified prescribe schema, no evidra binary needed.
		evidence, evErr := agent.NewSimpleProxyEvidence(evidenceDir)
		if evErr != nil {
			return nil, fmt.Errorf("harness: smart evidence: %w", evErr)
		}
		defer evidence.Close()
		loopExecutor = &agent.SmartToolExecutor{Base: executor, Evidence: evidence}
	} else if req.Config.ProxyMode {
		// Proxy mode: auto-record, agent unaware.
		proxyEvidence, proxyErr := agent.NewSimpleProxyEvidence(evidenceDir)
		if proxyErr != nil {
			return nil, fmt.Errorf("harness: proxy evidence: %w", proxyErr)
		}
		defer proxyEvidence.Close()
		executor.ProxyEvidence = proxyEvidence
	}

	loopResult, err := agent.RunLoop(agentCtx, agent.LoopConfig{
		Provider:     provider,
		Executor:     loopExecutor,
		Model:        req.Config.Model,
		MaxTurns:     25,
		MemoryWindow: req.Config.MemoryWindow,
		SystemPrompt: systemPrompt,
		TaskPrompt:   promptContent,
	})
	if err != nil {
		return nil, fmt.Errorf("harness: agent loop: %w", err)
	}

	// Build transcript from messages
	var transcript strings.Builder
	for _, m := range loopResult.Messages {
		transcript.WriteString(fmt.Sprintf("[%s] %s\n", m.Role, truncateForLog(m.Content, 500)))
		for _, tc := range m.ToolCalls {
			transcript.WriteString(fmt.Sprintf("  -> %s(%s)\n", tc.Name, truncateForLog(tc.Arguments, 200)))
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
		Metadata:   buildRunMetadata(req.Config, loopResult, evidenceDir),
	}, nil
}

// buildRunMetadata creates the metadata map for a provider-path run,
// including all version information for reproducibility.
func buildRunMetadata(cfg config.Config, loopResult *agent.LoopResult, evidenceDir string) map[string]string {
	evidenceMode := "direct"
	if cfg.SmartPrescribe {
		evidenceMode = "smart"
	} else if cfg.ProxyMode {
		evidenceMode = "proxy"
	} else if cfg.ResolveEvidraBin() == "" && cfg.ResolveSystemPromptFile() == "" {
		evidenceMode = "none"
	}

	meta := map[string]string{
		"provider":          cfg.Provider,
		"model":             cfg.Model,
		"evidence_mode":     evidenceMode,
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

// buildSystemPrompt loads the system prompt from file or returns the default.
func buildSystemPrompt(cfg config.Config, s *scenario.Scenario) (string, error) {
	promptFile := cfg.ResolveSystemPromptFile()
	if promptFile != "" {
		data, err := os.ReadFile(promptFile)
		if err != nil {
			return "", fmt.Errorf("harness: read system prompt file: %w", err)
		}
		prompt := string(data)
		// Append namespace context
		prompt += fmt.Sprintf("\n\nTarget namespace: %s\n", strings.Join(s.Scope.Namespaces, ", "))
		return prompt, nil
	}

	// Smart prescribe mode: auto-load the smart prescribe skill if no prompt file given.
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
			ns = "bench"
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
