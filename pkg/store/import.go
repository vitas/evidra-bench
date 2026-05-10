package store

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// runJSON matches the run.json artifact format.
type runJSON struct {
	ScenarioID string            `json:"scenario_id"`
	Adapter    string            `json:"adapter"`
	StartTime  time.Time         `json:"start_time"`
	EndTime    time.Time         `json:"end_time"`
	ExitCode   int               `json:"exit_code"`
	Passed     bool              `json:"passed"`
	Checks     json.RawMessage   `json:"checks"`
	Metadata   map[string]string `json:"metadata"`
}

// ImportFromArtifacts walks runsDir for run.json files and inserts records
// into the database. Returns the number of records imported.
func (s *Store) ImportFromArtifacts(runsDir string) (int, error) {
	count := 0
	err := filepath.WalkDir(runsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "run.json" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil // skip unreadable files
		}

		var rj runJSON
		if err := json.Unmarshal(data, &rj); err != nil {
			return nil // skip unparseable files
		}
		if rj.ScenarioID == "" {
			return nil
		}

		artifactDir := filepath.Dir(path)
		checksJSON := string(rj.Checks)
		metaJSON, _ := json.Marshal(rj.Metadata)

		checksPassed, checksTotal := countChecksFromJSON(checksJSON)

		// Determine evidence mode from metadata.
		// - Explicit evidence_mode in metadata takes priority
		// - skill_version present → mcp (older artifacts used protocol skills)
		// - Otherwise → none (baseline run, no evidence recording)
		evidenceMode := "none"
		if rj.Metadata["evidence_mode"] != "" {
			evidenceMode = rj.Metadata["evidence_mode"]
		} else if rj.Metadata["skill_version"] != "" {
			evidenceMode = "mcp"
		}

		rec := RunRecord{
			ID:                buildRunID(rj, artifactDir),
			ScenarioID:        rj.ScenarioID,
			Model:             rj.Metadata["model"],
			Provider:          rj.Metadata["provider"],
			Adapter:           rj.Adapter,
			EvidenceMode:      evidenceMode,
			ToolServer:        rj.Metadata["tool_server"],
			ToolServerVersion: rj.Metadata["tool_server_version"],
			Passed:            rj.Passed,
			Duration:          rj.EndTime.Sub(rj.StartTime).Seconds(),
			ExitCode:          rj.ExitCode,
			Turns:             parseInt(rj.Metadata["turns"]),
			MemoryWindow:      parseInt(rj.Metadata["memory_window"]),
			PromptTokens:      parseInt(rj.Metadata["prompt_tokens"]),
			CompletionTokens:  parseInt(rj.Metadata["completion_tokens"]),
			EstimatedCost:     parseFloat(rj.Metadata["estimated_cost"]),
			ChecksPassed:      checksPassed,
			ChecksTotal:       checksTotal,
			ChecksJSON:        checksJSON,
			MetadataJSON:      string(metaJSON),
			ArtifactDir:       artifactDir,
			CreatedAt:         rj.StartTime,
		}

		if err := s.Insert(rec); err != nil {
			return nil // skip duplicates
		}
		count++
		return nil
	})
	if err != nil {
		return count, fmt.Errorf("store.ImportFromArtifacts: %w", err)
	}
	return count, nil
}

func buildRunID(rj runJSON, artifactDir string) string {
	// Use directory name as a stable ID
	dirName := filepath.Base(artifactDir)
	if dirName != "" && dirName != "." {
		return dirName
	}
	return fmt.Sprintf("%s-%s-%s", rj.StartTime.Format("20060102-150405"), rj.ScenarioID, rj.Adapter)
}

type checksPayload struct {
	Checks []struct {
		Verdict string `json:"verdict"`
	} `json:"checks"`
}

func countChecksFromJSON(checksJSON string) (passed, total int) {
	if strings.TrimSpace(checksJSON) == "" {
		return 0, 0
	}
	var cp checksPayload
	if err := json.Unmarshal([]byte(checksJSON), &cp); err != nil {
		return 0, 0
	}
	for _, c := range cp.Checks {
		total++
		if c.Verdict == "pass" {
			passed++
		}
	}
	return
}

func parseInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func parseFloat(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err == nil {
		return f
	}
	// Handle "$0.0287 (in: ...)" format: extract first number after $
	if idx := strings.Index(s, "$"); idx >= 0 {
		num := s[idx+1:]
		// Take until space or paren
		for i, c := range num {
			if c == ' ' || c == '(' {
				num = num[:i]
				break
			}
		}
		f, _ = strconv.ParseFloat(num, 64)
		return f
	}
	return 0
}
