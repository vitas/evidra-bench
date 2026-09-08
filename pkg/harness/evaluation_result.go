package harness

import (
	"encoding/json"
	"time"

	"github.com/vitas/evidra-bench/pkg/adapter"
	"github.com/vitas/evidra-bench/pkg/autopsy"
	"github.com/vitas/evidra-bench/pkg/evaluation"
	"github.com/vitas/evidra-bench/pkg/verifier"
)

func buildEvaluationCaseResult(
	scenarioID string,
	runID string,
	agentResult *adapter.RunResult,
	verifyResult *verifier.VerifyResult,
	autopsyJSON json.RawMessage,
	artifactDir string,
	duration time.Duration,
	termination evaluation.Termination,
) evaluation.CaseResult {
	result := evaluation.CaseResult{
		ScenarioID:  scenarioID,
		RunID:       runID,
		Duration:    duration,
		Termination: termination,
	}
	if agentResult != nil {
		result.ExitCode = agentResult.ExitCode
		if agentResult.Metadata != nil {
			_, hasPrompt := agentResult.Metadata["prompt_tokens"]
			_, hasCompletion := agentResult.Metadata["completion_tokens"]
			result.Usage = evaluation.Usage{
				Known:            hasPrompt || hasCompletion,
				PromptTokens:     parseIntMeta(agentResult.Metadata, "prompt_tokens"),
				CompletionTokens: parseIntMeta(agentResult.Metadata, "completion_tokens"),
			}
		}
	}
	if verifyResult != nil {
		result.ChecksPassed, result.ChecksTotal = countChecks(verifyResult)
	}
	if artifactDir != "" {
		result.Evidence = []evaluation.EvidenceRef{{Kind: "artifact_dir", Path: artifactDir}}
	}

	var report autopsy.Report
	if len(autopsyJSON) > 0 && json.Unmarshal(autopsyJSON, &report) == nil {
		for _, finding := range report.Findings {
			if finding.Kind != autopsy.FailureUnsafeAction {
				continue
			}
			severity := evaluation.SeverityInfo
			switch finding.Severity {
			case autopsy.SeverityCritical:
				severity = evaluation.SeverityCritical
			case autopsy.SeverityWarning:
				severity = evaluation.SeverityWarning
			}
			result.Findings = append(result.Findings, evaluation.SafetyFinding{
				Kind:     string(finding.Kind),
				Severity: severity,
				Measured: finding.Severity == autopsy.SeverityCritical,
				Message:  finding.Message,
			})
		}
	}

	completed := termination.Kind == evaluation.TerminationComplete
	passed := verifyResult != nil && verifyResult.Passed
	result.Verdict = evaluation.ClassifyCaseVerdict(evaluation.CaseClassification{
		Completed:      completed,
		Passed:         passed,
		SafetyFindings: result.Findings,
	})
	return result
}
