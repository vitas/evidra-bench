// Package harness orchestrates the benchmark run loop.
package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"samebits.com/evidra-infra-bench/pkg/adapter"
	"samebits.com/evidra-infra-bench/pkg/artifact"
	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/environment"
	"samebits.com/evidra-infra-bench/pkg/report"
	"samebits.com/evidra-infra-bench/pkg/scenario"
	"samebits.com/evidra-infra-bench/pkg/verifier"
)

// Deps holds all dependencies for the harness.
type Deps struct {
	EnvProvider  environment.Provider
	Bootstrapper *environment.Bootstrapper
	Runner       environment.CommandRunner
	Adapter      adapter.Adapter
	Writer       *artifact.Writer
	Reporter     *report.Reporter
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
	if s.Break.Path != "" {
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

	agentResult, err := h.deps.Adapter.Run(ctx, adapter.RunInput{
		ScenarioID:     s.ID,
		PromptPath:     s.Prompt,
		WorkspaceDir:   req.Config.RunsDir,
		KubeconfigPath: handle.KubeconfigPath,
		Timeout:        timeout,
		AgentCommand:   req.Config.AgentCommand,
	})
	if err != nil {
		return nil, fmt.Errorf("harness.Run: execute agent: %w", err)
	}

	// Step 5: Verify outcome.
	var checkDefs []verifier.CheckDef
	for _, c := range s.Checks {
		checkDefs = append(checkDefs, verifier.CheckDef{
			Type:      c.Type,
			Namespace: c.Namespace,
			Name:      c.Name,
		})
	}
	checkers, err := verifier.BuildCheckers(checkDefs)
	if err != nil {
		return nil, fmt.Errorf("harness.Run: build checkers: %w", err)
	}
	verifyResult := verifier.RunChecks(ctx, handle.KubeconfigPath, checkers)

	// Step 5b: Verify Evidra protocol compliance.
	if s.Evidra.Enabled {
		evidenceDir := req.Config.EvidraEvidenceDir
		if evidenceDir == "" {
			evidenceDir = filepath.Join(req.Config.RunsDir, "evidence")
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

	bundle := artifact.RunBundle{
		ScenarioID: s.ID,
		Adapter:    req.Config.Adapter,
		StartTime:  startTime,
		EndTime:    endTime,
		ExitCode:   agentResult.ExitCode,
		Passed:     verifyResult.Passed,
		Prompt:     promptContent,
		Transcript: agentResult.Transcript,
		Stdout:     agentResult.Stdout,
		Stderr:     agentResult.Stderr,
		ToolCalls:  toolCallsJSON,
		Checks:     checksJSON,
		Metadata:   agentResult.Metadata,
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
			},
		}
		if err := h.deps.Reporter.Report(entries); err != nil {
			log.Printf("[harness] warning: evidra report failed: %v", err)
		}
	}

	return &RunResult{
		ScenarioID:  s.ID,
		Passed:      verifyResult.Passed,
		ExitCode:    agentResult.ExitCode,
		Duration:    endTime.Sub(startTime),
		ArtifactDir: artifactDir,
		Checks:      verifyResult,
	}, nil
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

func breakCommandArgs(kubeconfigPath string, s *scenario.Scenario) ([]string, error) {
	switch s.Break.Type {
	case "", "apply", "kubectl-apply":
		if s.Break.Path == "" {
			return nil, fmt.Errorf("break fixture path is required for %q", s.Break.Type)
		}
		return []string{"kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", s.Break.Path}, nil
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
