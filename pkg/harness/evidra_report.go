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

// evidraIngestRequest extends RunRecord with optional artifacts for the evidra API.
type evidraIngestRequest struct {
	store.RunRecord
	Transcript string `json:"transcript,omitempty"`
	ToolCalls  any    `json:"tool_calls,omitempty"`
}

// ReportToEvidra posts a run record with artifacts to the evidra API bench ingest endpoint.
func ReportToEvidra(evidraURL, apiKey string, rec store.RunRecord, transcript string, toolCalls any) {
	if evidraURL == "" || apiKey == "" {
		return
	}

	payload := evidraIngestRequest{
		RunRecord:  rec,
		Transcript: transcript,
		ToolCalls:  toolCalls,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[evidra-report] marshal: %v", err)
		return
	}

	url := evidraURL + "/v1/bench/runs"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Printf("[evidra-report] create request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[evidra-report] POST %s: %v", url, err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		log.Printf("[evidra-report] HTTP %d: %v", resp.StatusCode, result)
		return
	}

	log.Printf("[evidra-report] reported %s to %s", rec.ID, evidraURL)
}

// ReportBatchToEvidra posts multiple run records to the evidra API batch endpoint.
func ReportBatchToEvidra(evidraURL, apiKey string, records []store.RunRecord) error {
	if evidraURL == "" || apiKey == "" {
		return fmt.Errorf("evidra URL and API key required")
	}

	payload := map[string]any{"runs": records}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	url := evidraURL + "/v1/bench/runs/batch"
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
		json.NewDecoder(resp.Body).Decode(&result)
		return fmt.Errorf("HTTP %d: %v", resp.StatusCode, result)
	}

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	log.Printf("[evidra-report] batch: imported %v records", result["imported"])
	return nil
}
