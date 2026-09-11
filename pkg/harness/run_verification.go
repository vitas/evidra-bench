package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/vitas/evidra-bench/pkg/adapter"
	"github.com/vitas/evidra-bench/pkg/verifier"
)

func (h *Harness) verifyRun(ctx context.Context, req RunRequest, kubeconfigPath string, agentResult *adapter.RunResult, _ string, stageResults []StageResult, isMultiStage bool) (*verifier.VerifyResult, error) {
	return h.verifyRunPhase(ctx, req, kubeconfigPath, agentResult, stageResults, isMultiStage, "")
}

// verifyRunPhase is verifyRun with an explicit precondition phase. During
// ADR 0001 pre-flights EVIDRA_PHASE is exported to the check commands so a
// scenario's own scripts can tell healthy-baseline / broken / post-agent
// passes apart (cluster-side scripts see no other difference).
func (h *Harness) verifyRunPhase(ctx context.Context, req RunRequest, kubeconfigPath string, agentResult *adapter.RunResult, stageResults []StageResult, isMultiStage bool, phase string) (*verifier.VerifyResult, error) {
	s := req.Scenario
	if phase != "" {
		prev := os.Getenv("EVIDRA_PHASE")
		if err := os.Setenv("EVIDRA_PHASE", phase); err != nil {
			return nil, err
		}
		defer func() { _ = os.Setenv("EVIDRA_PHASE", prev) }()
	}
	var verifyResult *verifier.VerifyResult

	if isMultiStage {
		verifyResult = &verifier.VerifyResult{Passed: true}
		for _, sr := range stageResults {
			if !sr.Passed {
				verifyResult.Passed = false
			}
		}
	} else {
		checkDefs := checksToCheckDefs(s.Checks)
		checkers, err := verifier.BuildCheckers(checkDefs)
		if err != nil {
			return nil, fmt.Errorf("harness.Run: build checkers: %w", err)
		}
		verifyResult = verifier.RunChecks(ctx, kubeconfigPath, checkers)
	}

	addStageMetadata(agentResult, stageResults)
	return verifyResult, nil
}

func addStageMetadata(agentResult *adapter.RunResult, stageResults []StageResult) {
	if len(stageResults) == 0 {
		return
	}
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
