package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/vitas/evidra-bench/pkg/artifact"
	"github.com/vitas/evidra-bench/pkg/verifier"
)

// RunRecord is a historical run read from run.json, augmented with
// data derived from the run directory (scorecard.json) and the parsed
// verifier.VerifyResult. The JSON shape comes from artifact.RunBundle,
// so any schema change there is picked up here automatically.
type RunRecord struct {
	artifact.RunBundle
	Dir       string                 `json:"-"`
	Checks    *verifier.VerifyResult `json:"checks,omitempty"`
	Signals   map[string]int         `json:"-"`
	Score     float64                `json:"-"`
	ScoreBand string                 `json:"-"`
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
		var bundle artifact.RunBundle
		if json.Unmarshal(data, &bundle) != nil {
			return nil
		}
		rec := RunRecord{
			RunBundle: bundle,
			Dir:       filepath.Dir(path),
		}
		if len(bundle.Checks) > 0 {
			var vr verifier.VerifyResult
			if json.Unmarshal(bundle.Checks, &vr) == nil {
				rec.Checks = &vr
			}
		}
		loadScorecard(&rec)
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

// loadScorecard reads scorecard.json if present and populates signal data.
func loadScorecard(rec *RunRecord) {
	scPath := filepath.Join(rec.Dir, "scorecard.json")
	data, err := os.ReadFile(scPath)
	if err != nil {
		return
	}
	var sc struct {
		Score   float64        `json:"score"`
		Band    string         `json:"band"`
		Signals map[string]int `json:"signals"`
	}
	if json.Unmarshal(data, &sc) == nil {
		rec.Score = sc.Score
		rec.ScoreBand = sc.Band
		rec.Signals = sc.Signals
	}
}

// ActiveSignals returns signal names with count > 0.
func ActiveSignals(signals map[string]int) []string {
	if len(signals) == 0 {
		return nil
	}
	var active []string
	for name, count := range signals {
		if count > 0 {
			active = append(active, name)
		}
	}
	sort.Strings(active)
	return active
}
