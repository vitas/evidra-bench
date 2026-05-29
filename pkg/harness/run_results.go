package harness

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/vitas/evidra-bench/pkg/adapter"
	"github.com/vitas/evidra-bench/pkg/artifact"
	bench "github.com/vitas/evidra-bench/pkg/bench"
	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/localstore"
	"github.com/vitas/evidra-bench/pkg/report"
	"github.com/vitas/evidra-bench/pkg/verifier"
)

func (h *Harness) writeRunArtifacts(req RunRequest, agentResult *adapter.RunResult, verifyResult *verifier.VerifyResult, promptContent string, chaosRunner *ChaosRunner, recorder *runArtifactRecorder, startTime, endTime time.Time) (string, json.RawMessage) {
	s := req.Scenario
	checksJSON, _ := json.Marshal(verifyResult)
	toolCallsJSON := marshalToolCallsJSON(agentResult.ToolCalls)
	timelineJSON := buildTimelineJSON(toolCallsJSON)
	runEventsJSON := recorder.EventsJSON()
	checksPassedForAutopsy, checksTotalForAutopsy := countChecks(verifyResult)
	autopsyJSON := buildFailureAutopsyJSON(localstore.RunRecord{
		ScenarioID:       s.ID,
		Model:            req.Config.Model,
		Provider:         req.Config.Provider,
		Adapter:          req.Config.Adapter,
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
		Timeline:       timelineJSON,
		Checks:         checksJSON,
		Autopsy:        autopsyJSON,
		RunEvents:      runEventsJSON,
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

func (h *Harness) writeFailedRunArtifacts(req RunRequest, agentResult *adapter.RunResult, verifyResult *verifier.VerifyResult, promptContent string, chaosRunner *ChaosRunner, recorder *runArtifactRecorder, runErr error, startTime, endTime time.Time) string {
	exitCode := failedRunExitCode(runErr, agentResultExitCode(agentResult))
	agentResult = failedAgentResult(agentResult, exitCode)
	verifyResult = failedVerifyResult(verifyResult)

	checksJSON, _ := json.Marshal(verifyResult)
	toolCallsJSON := marshalToolCallsJSON(agentResult.ToolCalls)
	timelineJSON := buildTimelineJSON(toolCallsJSON)
	runEventsJSON := recorder.EventsJSON()
	runErrArtifact := buildRunErrorArtifact(runErr, recorder.CurrentPhase(), agentResult.ExitCode, endTime)
	runErrorJSON := buildRunErrorJSON(runErrArtifact)

	rec := buildRunRecord(req, agentResult, verifyResult, "", startTime, endTime)
	autopsyJSON := buildRunErrorAutopsyJSON(rec, runErrArtifact)

	chaosJSON, chaosLog := chaosArtifacts(chaosRunner)
	chaosStepCount := 0
	chaosMode := ""
	if chaosRunner != nil {
		summary := chaosRunner.Snapshot()
		chaosStepCount = len(summary.Events)
		chaosMode = summary.Mode
	}

	artifactDir := ""
	if h.deps.Writer != nil {
		bundle := artifact.RunBundle{
			ScenarioID:     req.Scenario.ID,
			Adapter:        req.Config.Adapter,
			StartTime:      startTime,
			EndTime:        endTime,
			ExitCode:       agentResult.ExitCode,
			Passed:         false,
			Prompt:         promptContent,
			Transcript:     agentResult.Transcript,
			Stdout:         agentResult.Stdout,
			Stderr:         agentResult.Stderr,
			ToolCalls:      toolCallsJSON,
			Timeline:       timelineJSON,
			Checks:         checksJSON,
			Autopsy:        autopsyJSON,
			RunError:       runErrorJSON,
			RunEvents:      runEventsJSON,
			ChaosEnabled:   chaosRunner != nil,
			ChaosMode:      chaosMode,
			ChaosStepCount: chaosStepCount,
			ChaosTimeline:  chaosJSON,
			ChaosLog:       chaosLog,
			Metadata:       agentResult.Metadata,
		}
		out, err := h.deps.Writer.Write(bundle)
		if err != nil {
			log.Printf("[harness] warning: failed-run artifact write failed: %v", err)
		} else {
			artifactDir = out.Path
		}
	}

	rec.ArtifactDir = artifactDir
	h.persistRun(req, rec, agentResult.Transcript, agentResult.ToolCalls, timelineJSON, autopsyJSON, runErrorJSON, runEventsJSON)
	return artifactDir
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

func (h *Harness) storeRun(req RunRequest, agentResult *adapter.RunResult, verifyResult *verifier.VerifyResult, artifactDir string, autopsyJSON json.RawMessage, recorder *runArtifactRecorder, startTime, endTime time.Time) {
	rec := buildRunRecord(req, agentResult, verifyResult, artifactDir, startTime, endTime)
	timelineJSON := buildTimelineJSONFromToolCalls(agentResult.ToolCalls)
	h.persistRun(req, rec, agentResult.Transcript, agentResult.ToolCalls, timelineJSON, autopsyJSON, nil, recorder.EventsJSON())
}

func buildRunRecord(req RunRequest, agentResult *adapter.RunResult, verifyResult *verifier.VerifyResult, artifactDir string, startTime, endTime time.Time) localstore.RunRecord {
	runCfg := req.Config
	toolServer, toolServerVersion := resolveToolServerIdentity(runCfg)
	if runCfg.MCPServer != "" || toolServer != "" || toolServerVersion != "" {
		if agentResult.Metadata == nil {
			agentResult.Metadata = map[string]string{}
		}
		if toolServer != "" {
			agentResult.Metadata["tool_server"] = toolServer
		}
		if runCfg.MCPServer != "" {
			agentResult.Metadata["tool_server_cmd"] = runCfg.MCPServer
		}
		if toolServerVersion != "" {
			agentResult.Metadata["tool_server_version"] = toolServerVersion
		}
	}
	if agentResult.Metadata == nil {
		agentResult.Metadata = map[string]string{}
	}
	for k, v := range config.CollectVersions(version, commit, runCfg).ToMetadata() {
		if v != "" && agentResult.Metadata[k] == "" {
			agentResult.Metadata[k] = v
		}
	}
	if reportID := strings.TrimSpace(runCfg.ReportID); reportID != "" {
		agentResult.Metadata["report_id"] = reportID
	}
	checksPassed, checksTotal := countChecks(verifyResult)
	checksJSON, _ := json.Marshal(verifyResult)
	metadataJSON, _ := json.Marshal(agentResult.Metadata)

	s := req.Scenario
	return localstore.RunRecord{
		ID:                buildRunID(startTime, s.ID, req.Config.Adapter),
		ScenarioID:        s.ID,
		Model:             req.Config.Model,
		Provider:          req.Config.Provider,
		Adapter:           req.Config.Adapter,
		ToolServer:        toolServer,
		ToolServerVersion: toolServerVersion,
		SkillID:           agentResult.Metadata["skill_id"],
		SkillVersion:      agentResult.Metadata["skill_version"],
		SkillSource:       agentResult.Metadata["skill_source"],
		SkillSHA256:       agentResult.Metadata["skill_sha256"],
		Passed:            verifyResult.Passed,
		Duration:          endTime.Sub(startTime).Seconds(),
		ExitCode:          agentResult.ExitCode,
		Turns:             parseIntMeta(agentResult.Metadata, "turns"),
		MemoryWindow:      req.Config.MemoryWindow,
		PromptTokens:      parseIntMeta(agentResult.Metadata, "prompt_tokens"),
		CompletionTokens:  parseIntMeta(agentResult.Metadata, "completion_tokens"),
		EstimatedCost:     parseFloatMeta(agentResult.Metadata, "estimated_cost"),
		ChecksPassed:      checksPassed,
		ChecksTotal:       checksTotal,
		ChecksJSON:        string(checksJSON),
		MetadataJSON:      string(metadataJSON),
		ArtifactDir:       artifactDir,
		CreatedAt:         startTime,
	}
}

func buildRunID(startTime time.Time, scenarioID, adapter string) string {
	return fmt.Sprintf("%s-%s-%s", startTime.Format("20060102-150405"), scenarioID, adapter)
}

func (h *Harness) persistRun(req RunRequest, rec localstore.RunRecord, transcript string, toolCalls []adapter.ToolCallRecord, timelineJSON, autopsyJSON, runErrorJSON, runEventsJSON json.RawMessage) {
	if h.deps.Store != nil {
		if err := h.deps.Store.Insert(rec); err != nil {
			log.Printf("[harness] warning: store insert failed: %v", err)
		}
	}
	ReportToBench(req.Config.BenchURL, req.Config.BenchAPIKey, rec, transcript, marshalToolCallsJSON(toolCalls), timelineJSON, autopsyJSON, runErrorJSON, runEventsJSON)
}

func agentResultExitCode(agentResult *adapter.RunResult) int {
	if agentResult == nil {
		return 0
	}
	return agentResult.ExitCode
}

func failedAgentResult(agentResult *adapter.RunResult, exitCode int) *adapter.RunResult {
	if agentResult == nil {
		return &adapter.RunResult{
			ExitCode: exitCode,
			Metadata: map[string]string{},
		}
	}
	clone := *agentResult
	if clone.ExitCode == 0 {
		clone.ExitCode = exitCode
	}
	if agentResult.Metadata != nil {
		clone.Metadata = make(map[string]string, len(agentResult.Metadata))
		for key, value := range agentResult.Metadata {
			clone.Metadata[key] = value
		}
	} else {
		clone.Metadata = map[string]string{}
	}
	return &clone
}

func failedVerifyResult(verifyResult *verifier.VerifyResult) *verifier.VerifyResult {
	if verifyResult != nil {
		verifyResult.Passed = false
		return verifyResult
	}
	return &verifier.VerifyResult{
		Passed: false,
		Checks: []verifier.CheckResult{},
	}
}

func marshalToolCallsJSON(toolCalls []adapter.ToolCallRecord) json.RawMessage {
	if toolCalls == nil {
		toolCalls = []adapter.ToolCallRecord{}
	}
	data, err := json.Marshal(toolCalls)
	if err != nil {
		log.Printf("[harness] warning: marshal tool calls: %v", err)
		return json.RawMessage(`[]`)
	}
	return data
}

func buildTimelineJSONFromToolCalls(toolCalls []adapter.ToolCallRecord) json.RawMessage {
	toolCallsJSON := marshalToolCallsJSON(toolCalls)
	return buildTimelineJSON(toolCallsJSON)
}

func buildTimelineJSON(toolCallsJSON json.RawMessage) json.RawMessage {
	var calls []bench.ToolCall
	if len(toolCallsJSON) > 0 {
		if err := json.Unmarshal(toolCallsJSON, &calls); err != nil {
			log.Printf("[harness] warning: timeline skipped: parse tool calls: %v", err)
			return nil
		}
	}
	data, err := json.MarshalIndent(bench.Parse(calls), "", "  ")
	if err != nil {
		log.Printf("[harness] warning: timeline skipped: marshal: %v", err)
		return nil
	}
	return data
}
