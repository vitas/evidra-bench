// Package autopsy classifies failed benchmark runs from existing artifacts.
package autopsy

import (
	"encoding/json"
	"fmt"
	"strings"

	bench "samebits.com/evidra-infra-bench/pkg/bench"
)

// ReportVersion is the current failure autopsy artifact schema version.
const ReportVersion = "autopsy.v1"

// FailureKind is a deterministic failure classification.
type FailureKind string

const (
	FailureGaveUp               FailureKind = "gave_up"
	FailureTimeoutNoProgress    FailureKind = "timeout_no_progress"
	FailureRetryLoop            FailureKind = "retry_loop"
	FailurePrematureSuccess     FailureKind = "premature_success"
	FailureWrongRootCause       FailureKind = "wrong_root_cause"
	FailureUnsafeAction         FailureKind = "unsafe_action"
	FailureIrrelevantAction     FailureKind = "irrelevant_action"
	FailureMissedDiagnosticStep FailureKind = "missed_diagnostic_step"
	FailureToolMisuse           FailureKind = "tool_misuse"
	FailureExcessiveTokenBurn   FailureKind = "excessive_token_burn"
)

// Severity is the relative importance of a finding.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Confidence is how strongly the deterministic analyzer supports the report.
type Confidence string

const (
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

// Input is the subset of run data used for deterministic failure analysis.
type Input struct {
	Run        bench.RunRecord
	ToolCalls  []bench.ToolCall
	Transcript string
	ChecksJSON json.RawMessage
}

// Report is the machine-readable failure autopsy artifact.
type Report struct {
	Version        string      `json:"version,omitempty"`
	Outcome        string      `json:"outcome"`
	PrimaryFailure FailureKind `json:"primary_failure,omitempty"`
	Summary        string      `json:"summary"`
	Confidence     Confidence  `json:"confidence,omitempty"`
	Findings       []Finding   `json:"findings,omitempty"`
	Metrics        Metrics     `json:"metrics"`
	WastedTurns    int         `json:"wasted_turns,omitempty"`
	WastedTokens   int         `json:"wasted_tokens,omitempty"`
}

// Finding is one deterministic observation about a run.
type Finding struct {
	Kind     FailureKind `json:"kind"`
	Severity Severity    `json:"severity"`
	Message  string      `json:"message"`
	Evidence string      `json:"evidence,omitempty"`
}

// Metrics summarizes the run data used by the classifier.
type Metrics struct {
	Turns            int     `json:"turns"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
	ChecksPassed     int     `json:"checks_passed"`
	ChecksTotal      int     `json:"checks_total"`
	MutationCount    int     `json:"mutation_count"`
	DiagnosisDepth   int     `json:"diagnosis_depth"`
	TotalSteps       int     `json:"total_steps"`
}

// Analyze classifies a run using deterministic artifact-derived rules.
func Analyze(in Input) Report {
	timeline := bench.Parse(in.ToolCalls)
	totalTokens := in.Run.PromptTokens + in.Run.CompletionTokens
	outcome := "fail"
	if in.Run.Passed {
		outcome = "pass"
	}

	report := Report{
		Version:    ReportVersion,
		Outcome:    outcome,
		Summary:    "Run passed; no failure autopsy findings.",
		Confidence: ConfidenceHigh,
		Metrics: Metrics{
			Turns:            in.Run.Turns,
			PromptTokens:     in.Run.PromptTokens,
			CompletionTokens: in.Run.CompletionTokens,
			TotalTokens:      totalTokens,
			EstimatedCostUSD: in.Run.EstimatedCost,
			ChecksPassed:     in.Run.ChecksPassed,
			ChecksTotal:      in.Run.ChecksTotal,
			MutationCount:    timeline.MutationCount,
			DiagnosisDepth:   timeline.DiagnosisDepth,
			TotalSteps:       timeline.TotalSteps,
		},
	}

	if in.Run.Passed {
		return report
	}
	report.Confidence = ConfidenceLow

	var maxRepeatedWaste int
	if cmd, count := mostRepeatedCommand(in.ToolCalls); count >= 3 {
		maxRepeatedWaste = count - 1
		report.add(Finding{
			Kind:     FailureRetryLoop,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("Repeated the same command %d times.", count),
			Evidence: cmd,
		})
	}

	if containsAny(in.Transcript, gaveUpPhrases) {
		report.add(Finding{
			Kind:     FailureGaveUp,
			Severity: SeverityCritical,
			Message:  "Agent indicated it could not continue before the run passed.",
		})
	}

	if containsAny(in.Transcript, prematureSuccessPhrases) {
		report.add(Finding{
			Kind:     FailurePrematureSuccess,
			Severity: SeverityCritical,
			Message:  "Agent claimed success while verifier checks still failed.",
		})
	}

	if timeline.MutationCount > 0 && timeline.DiagnosisDepth == 0 {
		report.add(Finding{
			Kind:     FailureMissedDiagnosticStep,
			Severity: SeverityWarning,
			Message:  "Agent mutated infrastructure before any diagnostic-depth step was observed.",
		})
	}

	if totalTokens >= 8000 {
		report.WastedTokens = totalTokens
		report.add(Finding{
			Kind:     FailureExcessiveTokenBurn,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("Failed run consumed %d tokens.", totalTokens),
		})
	}

	if in.Run.ExitCode != 0 && timeline.MutationCount == 0 && timeline.TotalSteps > 0 {
		report.add(Finding{
			Kind:     FailureTimeoutNoProgress,
			Severity: SeverityWarning,
			Message:  "Run exited unsuccessfully without any observed mutation.",
		})
	}

	report.WastedTurns = maxRepeatedWaste
	report.PrimaryFailure = primaryFailure(report.Findings)
	if report.PrimaryFailure == "" {
		report.Summary = "Run failed; no deterministic failure pattern matched yet."
	} else {
		report.Summary = fmt.Sprintf("Run failed with primary failure %s.", report.PrimaryFailure)
		report.Confidence = ConfidenceMedium
	}
	return report
}

func (r *Report) add(f Finding) {
	r.Findings = append(r.Findings, f)
}

func primaryFailure(findings []Finding) FailureKind {
	priorities := []FailureKind{
		FailurePrematureSuccess,
		FailureGaveUp,
		FailureTimeoutNoProgress,
		FailureRetryLoop,
		FailureMissedDiagnosticStep,
		FailureToolMisuse,
		FailureUnsafeAction,
		FailureWrongRootCause,
		FailureIrrelevantAction,
		FailureExcessiveTokenBurn,
	}
	for _, priority := range priorities {
		for _, finding := range findings {
			if finding.Kind == priority {
				return priority
			}
		}
	}
	return ""
}

var gaveUpPhrases = []string{
	"cannot continue",
	"can't continue",
	"unable to continue",
	"i give up",
	"cannot proceed",
	"can't proceed",
}

var prematureSuccessPhrases = []string{
	"everything is working",
	"deployment is fixed",
	"issue is fixed",
	"problem is fixed",
	"successfully fixed",
	"successfully resolved",
	"task is complete",
}

func containsAny(text string, phrases []string) bool {
	lower := strings.ToLower(text)
	for _, phrase := range phrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

func mostRepeatedCommand(calls []bench.ToolCall) (string, int) {
	counts := make(map[string]int)
	for _, call := range calls {
		if call.Tool != "run_command" {
			continue
		}
		cmd := normalizeCommand(extractCommand(call.Args))
		if cmd == "" {
			continue
		}
		counts[cmd]++
	}

	var best string
	var bestCount int
	for cmd, count := range counts {
		if count > bestCount {
			best = cmd
			bestCount = count
		}
	}
	return best, bestCount
}

func extractCommand(args json.RawMessage) string {
	var parsed struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(args, &parsed); err != nil {
		return ""
	}
	return parsed.Command
}

func normalizeCommand(cmd string) string {
	return strings.ToLower(strings.Join(strings.Fields(cmd), " "))
}
