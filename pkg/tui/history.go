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

// LoadHistory reads all run.json files under the runs directory, recursively.
func LoadHistory(runsDir string) []RunRecord {
	var records []RunRecord
	_ = filepath.WalkDir(runsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "run.json" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		var rec RunRecord
		if json.Unmarshal(data, &rec) != nil {
			return nil
		}
		rec.Dir = filepath.Dir(path)
		if len(rec.RawChecks) > 0 {
			var vr verifier.VerifyResult
			if json.Unmarshal(rec.RawChecks, &vr) == nil {
				rec.Checks = &vr
			}
		}
		records = append(records, rec)
		return nil
	})
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
