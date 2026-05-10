package harness

import (
	"encoding/json"
	"log"

	"samebits.com/evidra-infra-bench/pkg/autopsy"
	bench "samebits.com/evidra-infra-bench/pkg/bench"
	"samebits.com/evidra-infra-bench/pkg/scenario"
	"samebits.com/evidra-infra-bench/pkg/store"
)

func buildFailureAutopsyJSON(rec store.RunRecord, toolCallsJSON json.RawMessage, transcript string, checksJSON json.RawMessage, hints scenario.AutopsyHints) json.RawMessage {
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
