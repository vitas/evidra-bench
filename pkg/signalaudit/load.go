package signalaudit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LoadRun reads a run artifact directory and extracts the signal data needed
// for auditing.
func LoadRun(runDir string) (Run, error) {
	data, err := os.ReadFile(filepath.Join(runDir, "run.json"))
	if err != nil {
		return Run{}, fmt.Errorf("read run.json: %w", err)
	}

	var raw struct {
		ScenarioID string            `json:"scenario_id"`
		Metadata   map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Run{}, fmt.Errorf("decode run.json: %w", err)
	}

	counts, source, err := loadSignalCounts(runDir, raw.Metadata)
	if err != nil {
		return Run{}, err
	}

	return Run{
		RunDir:       runDir,
		ScenarioID:   strings.TrimSpace(raw.ScenarioID),
		Model:        strings.TrimSpace(raw.Metadata["model"]),
		Provider:     strings.TrimSpace(raw.Metadata["provider"]),
		Signals:      signalNames(counts),
		SignalCounts: counts,
		SignalSource: source,
	}, nil
}

func loadSignalCounts(runDir string, metadata map[string]string) (map[string]int, string, error) {
	if counts, ok, err := loadScorecardSignals(runDir); err != nil {
		return nil, "", fmt.Errorf("load scorecard: %w", err)
	} else if ok {
		return counts, "scorecard", nil
	}

	evidenceDir := strings.TrimSpace(metadata["evidence_dir"])
	if !hasSegments(evidenceDir) {
		fallback := filepath.Join(runDir, "evidra")
		if hasSegments(fallback) {
			evidenceDir = fallback
		}
	}

	counts, err := loadEvidenceSignals(evidenceDir)
	if err != nil {
		return nil, "", fmt.Errorf("load evidence signals: %w", err)
	}
	if len(counts) == 0 {
		return counts, "none", nil
	}
	return counts, "evidence", nil
}

func loadScorecardSignals(runDir string) (map[string]int, bool, error) {
	for _, path := range []string{
		filepath.Join(runDir, "scorecard.json"),
		filepath.Join(runDir, "evidra", "scorecard.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			continue
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil, false, err
		}

		var raw struct {
			SignalSummary map[string]any `json:"signal_summary"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, false, err
		}

		counts := make(map[string]int, len(raw.SignalSummary))
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
		return counts, true, nil
	}

	return nil, false, nil
}

func loadEvidenceSignals(evidenceDir string) (map[string]int, error) {
	counts := map[string]int{}
	if !hasSegments(evidenceDir) {
		return counts, nil
	}

	files, err := filepath.Glob(filepath.Join(evidenceDir, "segments", "*.jsonl"))
	if err != nil {
		return nil, err
	}
	for _, path := range files {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
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
				return nil, err
			}
			if entry.Type != "signal" {
				continue
			}

			var payload struct {
				SignalName string `json:"signal_name"`
			}
			if err := json.Unmarshal(entry.Payload, &payload); err != nil {
				_ = file.Close()
				return nil, err
			}
			if strings.TrimSpace(payload.SignalName) != "" {
				counts[payload.SignalName]++
			}
		}
		if err := scanner.Err(); err != nil {
			_ = file.Close()
			return nil, err
		}
		if err := file.Close(); err != nil {
			return nil, err
		}
	}

	return counts, nil
}

func signalNames(counts map[string]int) []string {
	if len(counts) == 0 {
		return nil
	}

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
