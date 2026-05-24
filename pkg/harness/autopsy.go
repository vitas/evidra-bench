package harness

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/vitas/evidra-bench/pkg/autopsy"
	bench "github.com/vitas/evidra-bench/pkg/bench"
	"github.com/vitas/evidra-bench/pkg/localstore"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

func buildFailureAutopsyJSON(rec localstore.RunRecord, toolCallsJSON json.RawMessage, transcript string, checksJSON json.RawMessage, hints scenario.AutopsyHints) json.RawMessage {
	var calls []bench.ToolCall
	if len(toolCallsJSON) > 0 {
		if err := json.Unmarshal(toolCallsJSON, &calls); err != nil {
			log.Printf("[harness] warning: failure autopsy skipped: parse tool calls: %v", err)
			return nil
		}
	}
	report := autopsy.Analyze(autopsy.Input{
		Run:        rec,
		ToolCalls:  calls,
		Transcript: transcript,
		ChecksJSON: checksJSON,
		Hints:      convertAutopsyHints(hints),
	})
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Printf("[harness] warning: failure autopsy skipped: marshal: %v", err)
		return nil
	}
	return data
}

func buildRunErrorAutopsyJSON(rec localstore.RunRecord, runErr runErrorArtifact) json.RawMessage {
	report := autopsy.Report{
		Version:        autopsy.ReportVersion,
		Outcome:        "fail",
		PrimaryFailure: autopsy.FailureKind("run_error"),
		Summary:        fmt.Sprintf("Run failed during %s before complete deterministic autopsy was available.", runErr.Phase),
		Confidence:     autopsy.ConfidenceMedium,
		Findings: []autopsy.Finding{
			{
				Kind:     autopsy.FailureKind("run_error"),
				Severity: autopsy.SeverityCritical,
				Message:  fmt.Sprintf("Run ended with %s.", runErr.Kind),
				Evidence: runErr.Message,
			},
		},
		Metrics: autopsy.Metrics{
			Turns:            rec.Turns,
			PromptTokens:     rec.PromptTokens,
			CompletionTokens: rec.CompletionTokens,
			TotalTokens:      rec.PromptTokens + rec.CompletionTokens,
			EstimatedCostUSD: rec.EstimatedCost,
			ChecksPassed:     rec.ChecksPassed,
			ChecksTotal:      rec.ChecksTotal,
		},
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Printf("[harness] warning: failure autopsy skipped: marshal run error: %v", err)
		return nil
	}
	return data
}

func convertAutopsyHints(hints scenario.AutopsyHints) autopsy.Hints {
	return autopsy.Hints{
		ExpectedDiagnostics: convertAutopsyPatterns(hints.ExpectedDiagnostics),
		AllowedMutations:    convertAutopsyPatterns(hints.AllowedMutations),
		ForbiddenActions:    convertAutopsyPatterns(hints.ForbiddenActions),
		RootCauseResources:  append([]string(nil), hints.RootCauseResources...),
	}
}

func convertAutopsyPatterns(patterns []scenario.AutopsyPattern) []autopsy.Pattern {
	if len(patterns) == 0 {
		return nil
	}
	converted := make([]autopsy.Pattern, 0, len(patterns))
	for _, pattern := range patterns {
		converted = append(converted, autopsy.Pattern{
			Kind:     pattern.Kind,
			Pattern:  pattern.Pattern,
			Reason:   pattern.Reason,
			Severity: pattern.Severity,
		})
	}
	return converted
}
