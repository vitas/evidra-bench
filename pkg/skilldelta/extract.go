package skilldelta

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"samebits.com/evidra-infra-bench/pkg/verifier"
)

// BuildPairResult loads two run artifact directories and returns a normalized
// skill-delta pair result.
func BuildPairResult(withoutSkillDir, withSkillDir string) (PairResult, error) {
	withoutRun, err := loadRunSnapshot(withoutSkillDir)
	if err != nil {
		return PairResult{}, fmt.Errorf("load without_skill run: %w", err)
	}
	withRun, err := loadRunSnapshot(withSkillDir)
	if err != nil {
		return PairResult{}, fmt.Errorf("load with_skill run: %w", err)
	}

	scenarioID := firstNonEmpty(withRun.ScenarioID, withoutRun.ScenarioID)
	if withoutRun.ScenarioID != "" && withRun.ScenarioID != "" && withoutRun.ScenarioID != withRun.ScenarioID {
		return PairResult{}, fmt.Errorf("scenario mismatch: %s vs %s", withoutRun.ScenarioID, withRun.ScenarioID)
	}

	pair := PairResult{
		ScenarioID:           scenarioID,
		Model:                firstNonEmpty(withRun.Model, withoutRun.Model),
		Provider:             firstNonEmpty(withRun.Provider, withoutRun.Provider),
		Repeat:               inferRepeat(withoutSkillDir, withSkillDir),
		WithoutSkill:         withoutRun.Snapshot,
		WithSkill:            withRun.Snapshot,
		VerdictDelta:         verdictDelta(withoutRun.Snapshot.Passed, withRun.Snapshot.Passed),
		DurationDeltaSeconds: withRun.Snapshot.DurationSeconds - withoutRun.Snapshot.DurationSeconds,
		CostDeltaUSD:         withRun.Snapshot.EstimatedCostUSD - withoutRun.Snapshot.EstimatedCostUSD,
		ComplianceDeltaPct:   withRun.Snapshot.Protocol.ComplianceRatePct - withoutRun.Snapshot.Protocol.ComplianceRatePct,
		ScoreDelta:           scoreValue(withRun.Snapshot.Scorecard.Score) - scoreValue(withoutRun.Snapshot.Scorecard.Score),
		TokenDelta: TokenDelta{
			PromptTokens:     withRun.Snapshot.PromptTokens - withoutRun.Snapshot.PromptTokens,
			CompletionTokens: withRun.Snapshot.CompletionTokens - withoutRun.Snapshot.CompletionTokens,
			TotalTokens:      withRun.Snapshot.TotalTokens - withoutRun.Snapshot.TotalTokens,
		},
		Paths: PairPaths{
			WithoutSkillRunDir:      withoutSkillDir,
			WithSkillRunDir:         withSkillDir,
			WithoutSkillTranscript:  withoutRun.TranscriptPath,
			WithSkillTranscript:     withRun.TranscriptPath,
			WithoutSkillEvidenceDir: withoutRun.EvidenceDir,
			WithSkillEvidenceDir:    withRun.EvidenceDir,
			WithoutSkillScorecard:   withoutRun.ScorecardPath,
			WithSkillScorecard:      withRun.ScorecardPath,
		},
	}

	return pair, nil
}

type loadedRun struct {
	ScenarioID     string
	Model          string
	Provider       string
	EvidenceDir    string
	ScorecardPath  string
	TranscriptPath string
	Snapshot       RunSnapshot
}

func loadRunSnapshot(runDir string) (loadedRun, error) {
	data, err := os.ReadFile(filepath.Join(runDir, "run.json"))
	if err != nil {
		return loadedRun{}, err
	}

	var raw struct {
		ScenarioID string            `json:"scenario_id"`
		Passed     bool              `json:"passed"`
		ExitCode   int               `json:"exit_code"`
		StartTime  time.Time         `json:"start_time"`
		EndTime    time.Time         `json:"end_time"`
		Checks     json.RawMessage   `json:"checks"`
		Metadata   map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return loadedRun{}, fmt.Errorf("decode run.json: %w", err)
	}

	meta := cloneMetadata(raw.Metadata)
	checkResult := parseVerifyResult(raw.Checks)
	checksPassed, checksTotal := countChecks(checkResult.Checks)
	protocolMetrics := buildProtocolMetrics(checkResult.Checks)

	evidenceDir := strings.TrimSpace(meta["evidence_dir"])
	if evidenceDir == "" {
		if fallback := filepath.Join(runDir, "evidra"); hasSegments(fallback) {
			evidenceDir = fallback
		}
	}
	evidenceSummary, err := parseEvidenceSummary(evidenceDir)
	if err != nil {
		return loadedRun{}, fmt.Errorf("parse evidence summary: %w", err)
	}
	protocolMetrics.PrescribeCount = evidenceSummary.PrescribeCount
	protocolMetrics.ReportCount = evidenceSummary.ReportCount
	protocolMetrics.OrphanedPrescriptions = evidenceSummary.OrphanedPrescriptions
	protocolMetrics.DeclinedCount = evidenceSummary.DeclinedCount
	protocolMetrics.VerdictCoveragePct = evidenceSummary.VerdictCoveragePct
	if protocolMetrics.ChecksTotal > 0 {
		protocolMetrics.ComplianceRatePct = percentage(protocolMetrics.ChecksPassed, protocolMetrics.ChecksTotal)
	}

	scorecard, scorecardPath, err := loadScorecard(runDir)
	if err != nil {
		return loadedRun{}, fmt.Errorf("load scorecard: %w", err)
	}
	if len(scorecard.SignalCounts) == 0 && len(evidenceSummary.SignalCounts) > 0 {
		scorecard.SignalCounts = evidenceSummary.SignalCounts
		scorecard.Signals = signalNames(evidenceSummary.SignalCounts)
	}

	snapshot := RunSnapshot{
		RunID:            filepath.Base(runDir),
		Passed:           raw.Passed,
		ExitCode:         raw.ExitCode,
		DurationSeconds:  raw.EndTime.Sub(raw.StartTime).Seconds(),
		Turns:            parseInt(meta, "turns"),
		PromptTokens:     parseInt(meta, "prompt_tokens"),
		CompletionTokens: parseInt(meta, "completion_tokens"),
		EstimatedCostUSD: parseFloat(meta, "estimated_cost"),
		ChecksPassed:     checksPassed,
		ChecksTotal:      checksTotal,
		Protocol:         protocolMetrics,
		Scorecard:        scorecard,
		Metadata:         meta,
	}
	snapshot.TotalTokens = snapshot.PromptTokens + snapshot.CompletionTokens

	return loadedRun{
		ScenarioID:     raw.ScenarioID,
		Model:          strings.TrimSpace(meta["model"]),
		Provider:       strings.TrimSpace(meta["provider"]),
		EvidenceDir:    evidenceDir,
		ScorecardPath:  scorecardPath,
		TranscriptPath: filepath.Join(runDir, "transcript.txt"),
		Snapshot:       snapshot,
	}, nil
}

func parseVerifyResult(raw json.RawMessage) verifier.VerifyResult {
	if len(raw) == 0 {
		return verifier.VerifyResult{}
	}
	var vr verifier.VerifyResult
	if err := json.Unmarshal(raw, &vr); err != nil {
		return verifier.VerifyResult{}
	}
	return vr
}

func countChecks(checks []verifier.CheckResult) (passed, total int) {
	for _, check := range checks {
		total++
		if check.Verdict == verifier.VerdictPass {
			passed++
		}
	}
	return passed, total
}

func buildProtocolMetrics(checks []verifier.CheckResult) ProtocolMetrics {
	var metrics ProtocolMetrics
	for _, check := range checks {
		if check.Type != "evidra-protocol" {
			continue
		}
		metrics.ChecksTotal++
		if check.Verdict == verifier.VerdictPass {
			metrics.ChecksPassed++
		}
	}
	return metrics
}

type evidenceSummary struct {
	PrescribeCount        int
	ReportCount           int
	OrphanedPrescriptions int
	DeclinedCount         int
	VerdictCoveragePct    float64
	SignalCounts          map[string]int
}

func parseEvidenceSummary(evidenceDir string) (evidenceSummary, error) {
	if strings.TrimSpace(evidenceDir) == "" || !hasSegments(evidenceDir) {
		return evidenceSummary{SignalCounts: map[string]int{}}, nil
	}

	files, err := filepath.Glob(filepath.Join(evidenceDir, "segments", "*.jsonl"))
	if err != nil {
		return evidenceSummary{}, err
	}

	prescriptions := map[string]struct{}{}
	reports := map[string]struct{}{}
	summary := evidenceSummary{SignalCounts: map[string]int{}}
	reportsWithVerdict := 0

	for _, path := range files {
		file, err := os.Open(path)
		if err != nil {
			return evidenceSummary{}, err
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			var entry struct {
				Type    string          `json:"type"`
				Payload json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				_ = file.Close()
				return evidenceSummary{}, err
			}

			switch entry.Type {
			case "prescribe":
				summary.PrescribeCount++
				var payload struct {
					PrescriptionID string `json:"prescription_id"`
				}
				if json.Unmarshal(entry.Payload, &payload) == nil && payload.PrescriptionID != "" {
					prescriptions[payload.PrescriptionID] = struct{}{}
				}
			case "report":
				summary.ReportCount++
				var payload struct {
					PrescriptionID string `json:"prescription_id"`
					Verdict        string `json:"verdict"`
				}
				if json.Unmarshal(entry.Payload, &payload) == nil {
					if payload.PrescriptionID != "" {
						reports[payload.PrescriptionID] = struct{}{}
					}
					if strings.TrimSpace(payload.Verdict) != "" {
						reportsWithVerdict++
					}
					if payload.Verdict == "declined" {
						summary.DeclinedCount++
					}
				}
			case "signal":
				var payload struct {
					SignalName string `json:"signal_name"`
				}
				if json.Unmarshal(entry.Payload, &payload) == nil && payload.SignalName != "" {
					summary.SignalCounts[payload.SignalName]++
				}
			}
		}
		if err := scanner.Err(); err != nil {
			_ = file.Close()
			return evidenceSummary{}, err
		}
		if err := file.Close(); err != nil {
			return evidenceSummary{}, err
		}
	}

	for prescriptionID := range prescriptions {
		if _, ok := reports[prescriptionID]; !ok {
			summary.OrphanedPrescriptions++
		}
	}
	if summary.ReportCount > 0 {
		summary.VerdictCoveragePct = percentage(reportsWithVerdict, summary.ReportCount)
	}
	return summary, nil
}

func loadScorecard(runDir string) (ScorecardMetrics, string, error) {
	for _, candidate := range []string{
		filepath.Join(runDir, "scorecard.json"),
		filepath.Join(runDir, "evidra", "scorecard.json"),
	} {
		if _, err := os.Stat(candidate); err != nil {
			continue
		}
		data, err := os.ReadFile(candidate)
		if err != nil {
			return ScorecardMetrics{}, "", err
		}
		var raw struct {
			Score         float64        `json:"score"`
			Band          string         `json:"band"`
			SignalSummary map[string]any `json:"signal_summary"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			return ScorecardMetrics{}, "", err
		}
		counts := map[string]int{}
		for name, value := range raw.SignalSummary {
			switch typed := value.(type) {
			case float64:
				counts[name] = int(typed)
			case map[string]any:
				if count, ok := typed["count"].(float64); ok {
					counts[name] = int(count)
				}
			}
		}
		score := raw.Score
		return ScorecardMetrics{
			Available:    true,
			Score:        &score,
			Band:         raw.Band,
			Signals:      signalNames(counts),
			SignalCounts: counts,
		}, candidate, nil
	}

	return ScorecardMetrics{Available: false, SignalCounts: map[string]int{}}, "", nil
}

func signalNames(counts map[string]int) []string {
	names := make([]string, 0, len(counts))
	for name, count := range counts {
		if count > 0 {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func hasSegments(dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, "segments"))
	return err == nil && info.IsDir()
}

func cloneMetadata(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func percentage(passed, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(passed) / float64(total) * 100
}

func parseInt(meta map[string]string, key string) int {
	value := strings.TrimSpace(meta[key])
	if value == "" {
		return 0
	}
	n, _ := strconv.Atoi(value)
	return n
}

func parseFloat(meta map[string]string, key string) float64 {
	value := strings.TrimSpace(meta[key])
	if value == "" {
		return 0
	}
	n, _ := strconv.ParseFloat(value, 64)
	return n
}

func scoreValue(score *float64) float64 {
	if score == nil {
		return 0
	}
	return *score
}

func verdictDelta(withoutPassed, withPassed bool) string {
	switch {
	case !withoutPassed && withPassed:
		return "improved"
	case withoutPassed && !withPassed:
		return "regressed"
	default:
		return "same"
	}
}

func inferRepeat(paths ...string) int {
	for _, path := range paths {
		for _, part := range strings.Split(filepath.Clean(path), string(filepath.Separator)) {
			if !strings.HasPrefix(part, "repeat-") {
				continue
			}
			n, err := strconv.Atoi(strings.TrimPrefix(part, "repeat-"))
			if err == nil {
				return n
			}
		}
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
