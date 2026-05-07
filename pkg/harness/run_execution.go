package harness

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"samebits.com/evidra-infra-bench/pkg/adapter"
	"samebits.com/evidra-infra-bench/pkg/agent"
	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/scenario"
)

func (h *Harness) injectSingleStageBreak(ctx context.Context, req RunRequest, kubeconfigPath string) error {
	s := req.Scenario
	if s.Break.Path != "" || s.Break.Type == "kubectl" || s.Break.Type == "shell" {
		if err := h.applyBreak(ctx, kubeconfigPath, s, req.ExtraEnv...); err != nil {
			return fmt.Errorf("harness.Run: inject break: %w", err)
		}
	}
	if h.deps.Bootstrapper != nil && len(s.AfterBreak) > 0 {
		plan := buildStepPlan(s.AfterBreak)
		if err := h.deps.Bootstrapper.Execute(ctx, plan, kubeconfigPath); err != nil {
			return fmt.Errorf("harness.Run: after_break: %w", err)
		}
	}
	return nil
}

type multiStageRunError struct {
	err error
}

func (e *multiStageRunError) Error() string { return e.err.Error() }
func (e *multiStageRunError) Unwrap() error { return e.err }

func wrapRunAgentError(err error) error {
	var multiStageErr *multiStageRunError
	if errors.As(err, &multiStageErr) {
		return fmt.Errorf("harness.Run: multi-stage: %w", multiStageErr.err)
	}
	return fmt.Errorf("harness.Run: execute agent: %w", err)
}

func prepareAgentExecution(cfg config.Config, s *scenario.Scenario) (string, time.Duration, error) {
	promptContent := ""
	if s.Prompt != "" {
		data, err := os.ReadFile(s.Prompt)
		if err != nil {
			return "", 0, fmt.Errorf("harness.Run: read prompt: %w", err)
		}
		promptContent = string(data)
	}

	timeout := cfg.Timeout
	if s.Timeout.Duration > 0 {
		timeout = s.Timeout.Duration
	}
	if err := os.MkdirAll(cfg.RunsDir, 0755); err != nil {
		return "", 0, fmt.Errorf("harness.Run: create runs dir: %w", err)
	}
	return promptContent, timeout, nil
}

type runChaosHandle struct {
	done   chan struct{}
	cancel context.CancelFunc
	runner *ChaosRunner
}

func (h *Harness) startRunChaos(ctx context.Context, s *scenario.Scenario, kubeconfigPath string) *runChaosHandle {
	if len(s.Chaos.Steps) == 0 {
		return nil
	}

	chaosCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	runner := bootstrapperRunner(h.deps.Bootstrapper)
	chaosRunner := &ChaosRunner{
		Runner:         runner,
		KubeconfigPath: kubeconfigPath,
		Config:         s.Chaos,
	}
	go func() {
		defer close(done)
		chaosRunner.Run(chaosCtx)
	}()

	return &runChaosHandle{
		done:   done,
		cancel: cancel,
		runner: chaosRunner,
	}
}

func runChaosRunner(c *runChaosHandle) *ChaosRunner {
	if c == nil {
		return nil
	}
	return c.runner
}

func (c *runChaosHandle) stopForAgentError(cfg scenario.ChaosConfig) {
	if c == nil {
		return
	}
	if c.cancel != nil && shouldCancelChaosOnAgentDone(cfg) {
		c.cancel()
	}
	if c.done != nil {
		<-c.done
	}
}

func (c *runChaosHandle) stopAfterAgentDone(cfg scenario.ChaosConfig) {
	if c == nil {
		return
	}
	if c.cancel != nil && shouldCancelChaosOnAgentDone(cfg) {
		c.cancel()
	}
	if c.done != nil {
		<-c.done
		if c.cancel != nil {
			c.cancel()
		}
	}
}

func (h *Harness) executeRunAgent(ctx context.Context, req RunRequest, kubeconfigPath, promptContent string, timeout time.Duration, startTime time.Time, isMultiStage bool) (*adapter.RunResult, string, []StageResult, error) {
	s := req.Scenario
	var providerEvDir string
	var stageResults []StageResult

	// Preserve the existing multi-stage provider loop behavior: stages run
	// concurrently only when provider evidence capture is active.
	if isMultiStage && shouldUseProviderEvidenceDir(req.Config) {
		providerEvDir = providerEvidenceDir(req.Config.EvidenceDir, req.Config.RunsDir, s.ID, startTime)

		injectChan := make(chan agent.Message, 4)
		memoryResetChan := make(chan int, 1)
		agentCtx, agentCancel := context.WithCancel(ctx)
		agentDone := make(chan struct{})
		var agentResult *adapter.RunResult
		var agentErr error

		go func() {
			defer close(agentDone)
			agentResult, agentErr = h.runWithProvider(agentCtx, req, s, kubeconfigPath, promptContent, timeout, providerEvDir, injectChan, memoryResetChan)
		}()

		stageErr := h.runMultiStage(ctx, s, kubeconfigPath, injectChan, memoryResetChan, &stageResults)
		agentCancel()
		<-agentDone
		close(injectChan)
		close(memoryResetChan)

		if stageErr != nil {
			return nil, providerEvDir, stageResults, &multiStageRunError{err: stageErr}
		}
		if agentErr != nil && agentResult == nil {
			return nil, providerEvDir, stageResults, agentErr
		}
		if agentResult == nil {
			agentResult = &adapter.RunResult{ExitCode: 1}
		}
		return agentResult, providerEvDir, stageResults, nil
	}

	if shouldUseProviderEvidenceDir(req.Config) {
		providerEvDir = providerEvidenceDir(req.Config.EvidenceDir, req.Config.RunsDir, s.ID, startTime)
	}
	agentResult, err := h.executeSingleAgent(ctx, req, s, kubeconfigPath, promptContent, timeout, providerEvDir)
	if err != nil {
		return nil, providerEvDir, stageResults, err
	}
	return agentResult, providerEvDir, stageResults, nil
}
