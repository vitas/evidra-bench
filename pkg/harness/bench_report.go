package harness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"samebits.com/evidra-infra-bench/pkg/store"
)

// benchIngestRequest extends RunRecord with optional artifacts for the bench API.
type benchIngestRequest struct {
	store.RunRecord
	Transcript string `json:"transcript,omitempty"`
	ToolCalls  any    `json:"tool_calls,omitempty"`
}

// ReportToBench posts a run record with artifacts to the bench API bench ingest endpoint.
func ReportToBench(benchURL, apiKey string, rec store.RunRecord, transcript string, toolCalls any) {
	if benchURL == "" || apiKey == "" {
		return
	}

	payload := benchIngestRequest{
		RunRecord:  rec,
		Transcript: transcript,
		ToolCalls:  toolCalls,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[bench-report] marshal: %v", err)
		return
	}

	url := benchURL + "/v1/bench/runs"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Printf("[bench-report] create request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[bench-report] POST %s: %v", url, err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			log.Printf("[bench-report] decode error response: %v", err)
		}
		log.Printf("[bench-report] HTTP %d: %v", resp.StatusCode, result)
		return
	}

	log.Printf("[bench-report] reported %s to %s", rec.ID, benchURL)
}

// ReportBatchToBench posts multiple run records to the bench API batch endpoint.
func ReportBatchToBench(benchURL, apiKey string, records []store.RunRecord) error {
	if benchURL == "" || apiKey == "" {
		return fmt.Errorf("bench URL and API key required")
	}

	payload := map[string]any{"runs": records}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	url := benchURL + "/v1/bench/runs/batch"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return fmt.Errorf("decode error response: %w", err)
		}
		return fmt.Errorf("HTTP %d: %v", resp.StatusCode, result)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode success response: %w", err)
	}
	log.Printf("[bench-report] batch: imported %v records", result["imported"])
	return nil
}
