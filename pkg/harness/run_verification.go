package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"samebits.com/evidra-infra-bench/pkg/adapter"
	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/verifier"
)

func (h *Harness) verifyRun(ctx context.Context, req RunRequest, kubeconfigPath string, agentResult *adapter.RunResult, providerEvDir string, stageResults []StageResult, isMultiStage bool) (*verifier.VerifyResult, error) {
	s := req.Scenario
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
	evidenceDir := resolveRunEvidenceDir(req.Config, providerEvDir)
	verifyEvidraEvidence(ctx, req, kubeconfigPath, evidenceDir, verifyResult)
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

func resolveRunEvidenceDir(cfg config.Config, providerEvDir string) string {
	if providerEvDir != "" {
		return providerEvDir
	}
	if cfg.EvidenceDir != "" {
		return cfg.EvidenceDir
	}
	return filepath.Join(cfg.RunsDir, "evidence")
}

func verifyEvidraEvidence(ctx context.Context, req RunRequest, kubeconfigPath, evidenceDir string, verifyResult *verifier.VerifyResult) {
	s := req.Scenario
	evidenceMode := config.EffectiveEvidenceMode(req.Config)
	if !s.Evidra.Enabled || evidenceMode == "none" || req.Config.EvidenceDir == "" {
		return
	}

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
	evidraResult := verifier.RunChecks(ctx, kubeconfigPath, evidraCheckers)
	verifyResult.Checks = append(verifyResult.Checks, evidraResult.Checks...)
	if !evidraResult.Passed {
		verifyResult.Passed = false
	}
}
