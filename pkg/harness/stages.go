package harness

import (
	"context"
	"fmt"
	"log"
	"time"

	"samebits.com/evidra-infra-bench/pkg/agent"
	"samebits.com/evidra-infra-bench/pkg/scenario"
	"samebits.com/evidra-infra-bench/pkg/verifier"
)

// StageResult records the outcome of one stage.
type StageResult struct {
	Name         string        `json:"name"`
	Passed       bool          `json:"passed"`
	ChecksPassed int           `json:"checks_passed"`
	ChecksTotal  int           `json:"checks_total"`
	Duration     time.Duration `json:"duration"`
}

// runMultiStage executes a multi-stage scenario concurrently with the agent loop.
// It injects breaks, sends agent_goal messages and memory changes via channels,
// then polls checks until they pass or the stage times out.
func (h *Harness) runMultiStage(
	ctx context.Context,
	s *scenario.Scenario,
	kubeconfigPath string,
	injectChan chan<- agent.Message,
	memoryResetChan chan<- int,
	stageResults *[]StageResult,
) error {
	for i, stage := range s.Stages {
		stageStart := time.Now()
		log.Printf("[stage %d/%d] %s — injecting break", i+1, len(s.Stages), stage.Name)

		// 1. Inject break — create a temporary scenario with the stage's break.
		stageBreakScenario := *s
		stageBreakScenario.Break = stage.Break
		if err := h.applyBreak(ctx, kubeconfigPath, &stageBreakScenario); err != nil {
			return fmt.Errorf("stage %q break: %w", stage.Name, err)
		}

		// 2. Run after_break steps.
		if h.deps.Bootstrapper != nil && len(stage.AfterBreak) > 0 {
			log.Printf("[stage %d/%d] running %d after_break steps", i+1, len(s.Stages), len(stage.AfterBreak))
			plan := buildStepPlan(stage.AfterBreak)
			if err := h.deps.Bootstrapper.Execute(ctx, plan, kubeconfigPath); err != nil {
				return fmt.Errorf("stage %q after_break: %w", stage.Name, err)
			}
		}

		// 3. Send memory change to agent (before agent_goal so context is ready).
		if stage.Break.Memory != "" && memoryResetChan != nil {
			window := 3 // compact: keep last 3 exchanges
			if stage.Break.Memory == "reset" {
				window = 0
			}
			select {
			case memoryResetChan <- window:
				log.Printf("[stage %d/%d] memory: %s (window=%d)", i+1, len(s.Stages), stage.Break.Memory, window)
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// 4. Send agent_goal as injected user message.
		if stage.AgentGoal != "" && injectChan != nil {
			msg := agent.Message{Role: "user", Content: stage.AgentGoal}
			select {
			case injectChan <- msg:
				log.Printf("[stage %d/%d] sent agent_goal: %s", i+1, len(s.Stages), truncate(stage.AgentGoal, 80))
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// 5. Build checkers and poll until pass or stage timeout.
		checkDefs := checksToCheckDefs(stage.Checks)
		checkers, err := verifier.BuildCheckers(checkDefs)
		if err != nil {
			return fmt.Errorf("stage %q build checkers: %w", stage.Name, err)
		}

		stageCtx := ctx
		var stageCancel context.CancelFunc
		if stage.Timeout.Set {
			stageCtx, stageCancel = context.WithTimeout(ctx, stage.Timeout.Duration)
		}

		result := verifier.PollChecks(stageCtx, kubeconfigPath, checkers, 5*time.Second)

		if stageCancel != nil {
			stageCancel()
		}

		sr := StageResult{
			Name:         stage.Name,
			Passed:       result.Passed,
			ChecksPassed: countPassed(result),
			ChecksTotal:  len(result.Checks),
			Duration:     time.Since(stageStart),
		}
		*stageResults = append(*stageResults, sr)

		log.Printf("[stage %d/%d] %s — %s (checks: %d/%d, %v)",
			i+1, len(s.Stages), stage.Name,
			verdictStr(result.Passed), sr.ChecksPassed, sr.ChecksTotal, sr.Duration)

		if !result.Passed {
			onFail := stage.OnFail
			if onFail == "" {
				onFail = "stop"
			}
			if onFail == "continue" {
				log.Printf("[stage %d/%d] %s failed, continuing (on_fail=continue)", i+1, len(s.Stages), stage.Name)
				continue
			}
			log.Printf("[stage %d/%d] %s failed, stopping (on_fail=%s)", i+1, len(s.Stages), stage.Name, onFail)
			return nil
		}
	}
	return nil
}

// checksToCheckDefs converts scenario checks to verifier check definitions.
func checksToCheckDefs(checks []scenario.Check) []verifier.CheckDef {
	defs := make([]verifier.CheckDef, len(checks))
	for i, c := range checks {
		defs[i] = verifier.CheckDef{
			Type:      c.Type,
			Namespace: c.Namespace,
			Name:      c.Name,
			Condition: c.Condition,
		}
	}
	return defs
}

func countPassed(r *verifier.VerifyResult) int {
	n := 0
	for _, c := range r.Checks {
		if c.Verdict == verifier.VerdictPass {
			n++
		}
	}
	return n
}

func verdictStr(passed bool) string {
	if passed {
		return "PASS"
	}
	return "FAIL"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
