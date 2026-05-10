package harness

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"samebits.com/evidra-infra-bench/pkg/adapter"
	"samebits.com/evidra-infra-bench/pkg/artifact"
	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/report"
	"samebits.com/evidra-infra-bench/pkg/store"
	"samebits.com/evidra-infra-bench/pkg/verifier"
)

func (h *Harness) writeRunArtifacts(req RunRequest, agentResult *adapter.RunResult, verifyResult *verifier.VerifyResult, promptContent string, chaosRunner *ChaosRunner, startTime, endTime time.Time) (string, json.RawMessage) {
	s := req.Scenario
	checksJSON, _ := json.Marshal(verifyResult)
	toolCallsJSON, _ := json.Marshal(agentResult.ToolCalls)
	checksPassedForAutopsy, checksTotalForAutopsy := countChecks(verifyResult)
	autopsyJSON := buildFailureAutopsyJSON(store.RunRecord{
		ScenarioID:       s.ID,
		Model:            req.Config.Model,
		Provider:         req.Config.Provider,
		Adapter:          req.Config.Adapter,
		EvidenceMode:     config.EffectiveEvidenceMode(req.Config),
		Passed:           verifyResult.Passed,
		Duration:         endTime.Sub(startTime).Seconds(),
		ExitCode:         agentResult.ExitCode,
		Turns:            parseIntMeta(agentResult.Metadata, "turns"),
		MemoryWindow:     req.Config.MemoryWindow,
		PromptTokens:     parseIntMeta(agentResult.Metadata, "prompt_tokens"),
		CompletionTokens: parseIntMeta(agentResult.Metadata, "completion_tokens"),
		EstimatedCost:    parseFloatMeta(agentResult.Metadata, "estimated_cost"),
		ChecksPassed:     checksPassedForAutopsy,
		ChecksTotal:      checksTotalForAutopsy,
		ChecksJSON:       string(checksJSON),
		CreatedAt:        startTime,
	}, toolCallsJSON, agentResult.Transcript, checksJSON, s.Autopsy)

	chaosJSON, chaosLog := chaosArtifacts(chaosRunner)
	chaosStepCount := 0
	chaosMode := ""
	if chaosRunner != nil {
		summary := chaosRunner.Snapshot()
		chaosStepCount = len(summary.Events)
		chaosMode = summary.Mode
	}

	bundle := artifact.RunBundle{
		ScenarioID:     s.ID,
		Adapter:        req.Config.Adapter,
		StartTime:      startTime,
		EndTime:        endTime,
		ExitCode:       agentResult.ExitCode,
		Passed:         verifyResult.Passed,
		Prompt:         promptContent,
		Transcript:     agentResult.Transcript,
		Stdout:         agentResult.Stdout,
		Stderr:         agentResult.Stderr,
		ToolCalls:      toolCallsJSON,
		Checks:         checksJSON,
		Autopsy:        autopsyJSON,
		ChaosEnabled:   chaosRunner != nil,
		ChaosMode:      chaosMode,
		ChaosStepCount: chaosStepCount,
		ChaosTimeline:  chaosJSON,
		ChaosLog:       chaosLog,
		Metadata:       agentResult.Metadata,
	}

	if h.deps.Writer == nil {
		return "", autopsyJSON
	}
	out, err := h.deps.Writer.Write(bundle)
	if err != nil {
		log.Printf("[harness] warning: artifact write failed: %v", err)
		return "", autopsyJSON
	}
	return out.Path, autopsyJSON
}

func (h *Harness) reportRun(req RunRequest, agentResult *adapter.RunResult, verifyResult *verifier.VerifyResult, startTime, endTime time.Time) {
	if h.deps.Reporter == nil {
		return
	}
	s := req.Scenario
	entries := []report.EvidenceEntry{
		{
			ID:         fmt.Sprintf("bench-%s-%d", s.ID, startTime.UnixMilli()),
			Type:       "benchmark-run",
			Actor:      req.Config.Adapter,
			Timestamp:  startTime,
			ScenarioID: s.ID,
			Adapter:    req.Config.Adapter,
			Passed:     verifyResult.Passed,
			ExitCode:   agentResult.ExitCode,
			Duration:   endTime.Sub(startTime),
			Metadata:   agentResult.Metadata,
		},
	}
	if err := h.deps.Reporter.Report(entries); err != nil {
		log.Printf("[harness] warning: offline report failed: %v", err)
	}
}

func (h *Harness) storeRun(req RunRequest, agentResult *adapter.RunResult, verifyResult *verifier.VerifyResult, artifactDir string, autopsyJSON json.RawMessage, startTime, endTime time.Time) {
	if h.deps.Store == nil {
		return
	}

	if req.Config.MCPServer != "" {
		agentResult.Metadata["tool_server"] = mcpServerName(req.Config.MCPServer)
		agentResult.Metadata["tool_server_cmd"] = req.Config.MCPServer
		if ver := mcpServerVersion(req.Config.MCPServer); ver != "" {
			agentResult.Metadata["tool_server_version"] = ver
		}
	}
	checksPassed, checksTotal := countChecks(verifyResult)
	checksJSON, _ := json.Marshal(verifyResult)
	metadataJSON, _ := json.Marshal(agentResult.Metadata)

	s := req.Scenario
	rec := store.RunRecord{
		ID:               fmt.Sprintf("%s-%s-%s", startTime.Format("20060102-150405"), s.ID, req.Config.Adapter),
		ScenarioID:       s.ID,
		Model:            req.Config.Model,
		Provider:         req.Config.Provider,
		Adapter:          req.Config.Adapter,
		EvidenceMode:     config.EffectiveEvidenceMode(req.Config),
		ToolServer:       mcpServerName(req.Config.MCPServer),
		Passed:           verifyResult.Passed,
		Duration:         endTime.Sub(startTime).Seconds(),
		ExitCode:         agentResult.ExitCode,
		Turns:            parseIntMeta(agentResult.Metadata, "turns"),
		MemoryWindow:     req.Config.MemoryWindow,
		PromptTokens:     parseIntMeta(agentResult.Metadata, "prompt_tokens"),
		CompletionTokens: parseIntMeta(agentResult.Metadata, "completion_tokens"),
		EstimatedCost:    parseFloatMeta(agentResult.Metadata, "estimated_cost"),
		ChecksPassed:     checksPassed,
		ChecksTotal:      checksTotal,
		ChecksJSON:       string(checksJSON),
		MetadataJSON:     string(metadataJSON),
		ArtifactDir:      artifactDir,
		CreatedAt:        startTime,
	}
	if err := h.deps.Store.Insert(rec); err != nil {
		log.Printf("[harness] warning: store insert failed: %v", err)
	}
	ReportToBench(req.Config.BenchURL, req.Config.BenchAPIKey, rec, agentResult.Transcript, agentResult.ToolCalls, autopsyJSON)
}
