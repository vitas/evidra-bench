package harness

import (
	"context"
	"fmt"
	"log"
	"time"

	"samebits.com/evidra-infra-bench/pkg/scenario"
	"samebits.com/evidra-infra-bench/pkg/verifier"
)

// StageResult records the outcome of one stage.
type StageResult struct {
	Name         string
	Passed       bool
	ChecksPassed int
	ChecksTotal  int
	Duration     time.Duration
}

// runMultiStage executes a multi-stage scenario by looping over stages:
// inject break -> optionally send agent_goal -> poll checks -> next stage.
func (h *Harness) runMultiStage(
	ctx context.Context,
	s *scenario.Scenario,
	kubeconfigPath string,
	stageResults *[]StageResult,
) error {
	for i, stage := range s.Stages {
		stageStart := time.Now()
		log.Printf("[stage %d/%d] %s — injecting break", i+1, len(s.Stages), stage.Name)

		// 1. Handle memory (logged but not enforced until agent supports it).
		if stage.Break.Memory != "" {
			log.Printf("[stage %d/%d] memory: %s (noted for agent)", i+1, len(s.Stages), stage.Break.Memory)
			// TODO: implement compact/reset on agent conversation
		}

		// 2. Inject break — create a temporary scenario with the stage's break.
		stageBreakScenario := *s
		stageBreakScenario.Break = stage.Break
		if err := h.applyBreak(ctx, kubeconfigPath, &stageBreakScenario); err != nil {
			return fmt.Errorf("stage %q break: %w", stage.Name, err)
		}

		// 3. Run after_break steps.
		if h.deps.Bootstrapper != nil && len(stage.AfterBreak) > 0 {
			log.Printf("[stage %d/%d] running %d after_break steps", i+1, len(s.Stages), len(stage.AfterBreak))
			plan := buildStepPlan(stage.AfterBreak)
			if err := h.deps.Bootstrapper.Execute(ctx, plan, kubeconfigPath); err != nil {
				return fmt.Errorf("stage %q after_break: %w", stage.Name, err)
			}
		}

		// 4. Send agent_goal if present.
		if stage.AgentGoal != "" {
			log.Printf("[stage %d/%d] agent_goal: %s", i+1, len(s.Stages), stage.AgentGoal)
			// TODO: send message to agent conversation via provider
		}

		// 5. Build checkers and poll.
		checkDefs := checksToCheckDefs(stage.Checks)
		checkers, err := verifier.BuildCheckers(checkDefs)
		if err != nil {
			return fmt.Errorf("stage %q build checkers: %w", stage.Name, err)
		}

		stageCtx := ctx
		if stage.Timeout.Set {
			var cancel context.CancelFunc
			stageCtx, cancel = context.WithTimeout(ctx, stage.Timeout.Duration)
			defer cancel()
		}

		result := verifier.PollChecks(stageCtx, kubeconfigPath, checkers, 5*time.Second)

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
