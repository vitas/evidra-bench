package benchsvc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	bench "samebits.com/evidra-infra-bench/pkg/bench"
)

// jsonlRecord matches the JSONL format from evidra-stand results.jsonl.
type jsonlRecord struct {
	ID               string  `json:"id"`
	ScenarioID       string  `json:"scenario_id"`
	Model            string  `json:"model"`
	Provider         string  `json:"provider"`
	Adapter          string  `json:"adapter"`
	Passed           bool    `json:"passed"`
	Duration         float64 `json:"duration_seconds"`
	ExitCode         int     `json:"exit_code"`
	Turns            int     `json:"turns"`
	MemoryWindow     int     `json:"memory_window"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	EstimatedCost    float64 `json:"estimated_cost_usd"`
	ChecksPassed     int     `json:"checks_passed"`
	ChecksTotal      int     `json:"checks_total"`
	ChecksJSON       string  `json:"checks_json"`
	MetadataJSON     string  `json:"metadata_json"`
	CreatedAt        string  `json:"created_at"`
}

// ImportJSONL reads a results.jsonl file and inserts all records into PostgreSQL.
// Returns (imported, skipped, error).
func (s *PgStore) ImportJSONL(ctx context.Context, tenantID string, path string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, fmt.Errorf("bench.ImportJSONL: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	var records []bench.RunRecord
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB lines

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var jr jsonlRecord
		if err := json.Unmarshal(line, &jr); err != nil {
			continue // skip malformed lines
		}
		if jr.ID == "" || jr.ScenarioID == "" || jr.Model == "" {
			continue
		}

		// Extract evidence_mode from metadata_json.
		evidenceMode := "none"
		if jr.MetadataJSON != "" {
			var meta map[string]string
			if err := json.Unmarshal([]byte(jr.MetadataJSON), &meta); err == nil {
				if m, ok := meta["evidence_mode"]; ok && m != "" {
					evidenceMode = m
				}
			}
		}

		// Parse created_at.
		createdAt := time.Now()
		if jr.CreatedAt != "" {
			if t, err := time.Parse(time.RFC3339, jr.CreatedAt); err == nil {
				createdAt = t
			} else if t, err := time.Parse(time.RFC3339Nano, jr.CreatedAt); err == nil {
				createdAt = t
			}
		}

		records = append(records, bench.RunRecord{
			ID:               jr.ID,
			TenantID:         tenantID,
			ScenarioID:       jr.ScenarioID,
			Model:            jr.Model,
			Provider:         jr.Provider,
			Adapter:          jr.Adapter,
			EvidenceMode:     evidenceMode,
			Passed:           jr.Passed,
			Duration:         jr.Duration,
			ExitCode:         jr.ExitCode,
			Turns:            jr.Turns,
			MemoryWindow:     jr.MemoryWindow,
			PromptTokens:     jr.PromptTokens,
			CompletionTokens: jr.CompletionTokens,
			EstimatedCost:    jr.EstimatedCost,
			ChecksPassed:     jr.ChecksPassed,
			ChecksTotal:      jr.ChecksTotal,
			ChecksJSON:       jr.ChecksJSON,
			MetadataJSON:     jr.MetadataJSON,
			CreatedAt:        createdAt,
		})
	}

	if err := scanner.Err(); err != nil {
		return 0, 0, fmt.Errorf("bench.ImportJSONL: scan: %w", err)
	}

	if len(records) == 0 {
		return 0, 0, nil
	}

	// Batch insert with ON CONFLICT DO NOTHING.
	count, err := s.InsertRunBatch(ctx, tenantID, records)
	if err != nil {
		return 0, 0, fmt.Errorf("bench.ImportJSONL: batch insert: %w", err)
	}

	return count, len(records) - count, nil
}
