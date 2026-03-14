package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"samebits.com/evidra-infra-bench/pkg/verifier"
)

// RunRecord is a single historical run read from run.json.
type RunRecord struct {
	ScenarioID string                 `json:"scenario_id"`
	Adapter    string                 `json:"adapter"`
	StartTime  time.Time              `json:"start_time"`
	EndTime    time.Time              `json:"end_time"`
	ExitCode   int                    `json:"exit_code"`
	Passed     bool                   `json:"passed"`
	Checks     *verifier.VerifyResult `json:"-"`
	RawChecks  json.RawMessage        `json:"checks"`
	Dir        string                 `json:"-"`
}

// Duration returns the run duration.
func (r RunRecord) Duration() time.Duration {
	return r.EndTime.Sub(r.StartTime)
}

// LoadHistory reads all run.json files from the runs directory.
func LoadHistory(runsDir string) []RunRecord {
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return nil
	}
	var records []RunRecord
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		runJSON := filepath.Join(runsDir, entry.Name(), "run.json")
		data, err := os.ReadFile(runJSON)
		if err != nil {
			continue
		}
		var rec RunRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			continue
		}
		rec.Dir = filepath.Join(runsDir, entry.Name())
		if len(rec.RawChecks) > 0 {
			var vr verifier.VerifyResult
			if json.Unmarshal(rec.RawChecks, &vr) == nil {
				rec.Checks = &vr
			}
		}
		records = append(records, rec)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].StartTime.After(records[j].StartTime)
	})
	return records
}

// HistoryForScenario returns runs for a specific scenario, most recent first.
func HistoryForScenario(records []RunRecord, scenarioID string) []RunRecord {
	var result []RunRecord
	for _, r := range records {
		if r.ScenarioID == scenarioID {
			result = append(result, r)
		}
	}
	return result
}

// ScenarioStats summarizes run history for a scenario.
type ScenarioStats struct {
	TotalRuns  int
	PassCount  int
	FailCount  int
	LastResult string // "pass", "fail", or ""
}

// ComputeStats computes stats for a scenario from history records.
func ComputeStats(records []RunRecord) ScenarioStats {
	stats := ScenarioStats{TotalRuns: len(records)}
	for _, r := range records {
		if r.Passed {
			stats.PassCount++
		} else {
			stats.FailCount++
		}
	}
	if len(records) > 0 {
		if records[0].Passed {
			stats.LastResult = "pass"
		} else {
			stats.LastResult = "fail"
		}
	}
	return stats
}
