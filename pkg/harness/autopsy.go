package harness

import (
	"encoding/json"
	"log"

	"samebits.com/evidra-infra-bench/pkg/autopsy"
	bench "samebits.com/evidra-infra-bench/pkg/bench"
	"samebits.com/evidra-infra-bench/pkg/store"
)

func buildFailureAutopsyJSON(rec store.RunRecord, toolCallsJSON json.RawMessage, transcript string, checksJSON json.RawMessage) json.RawMessage {
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
	})
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Printf("[harness] warning: failure autopsy skipped: marshal: %v", err)
		return nil
	}
	return data
}
