// Package report provides optional Evidra reporting for benchmark runs.
package report

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	evidraclient "samebits.com/evidra/pkg/client"
)

// Config configures the Evidra reporter.
type Config struct {
	EvidencePath string
	EvidraURL    string
	EvidraAPIKey string
}

// IsOnline returns true if online reporting is configured.
func (c *Config) IsOnline() bool {
	return c.EvidraURL != "" && c.EvidraAPIKey != ""
}

// EvidenceEntry represents a single evidence record in Evidra format.
type EvidenceEntry struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Actor      string            `json:"actor"`
	Timestamp  time.Time         `json:"timestamp"`
	ScenarioID string            `json:"scenario_id"`
	Adapter    string            `json:"adapter"`
	Passed     bool              `json:"passed"`
	ExitCode   int               `json:"exit_code"`
	Duration   time.Duration     `json:"duration_ns"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// Reporter writes Evidra evidence to local files and optionally uploads.
type Reporter struct {
	cfg Config
}

// NewReporter creates a Reporter with the given config.
func NewReporter(cfg Config) *Reporter {
	return &Reporter{cfg: cfg}
}

// WriteOffline writes evidence entries as JSONL to the evidence directory.
func (r *Reporter) WriteOffline(entries []EvidenceEntry) error {
	if r.cfg.EvidencePath == "" {
		return fmt.Errorf("report.Reporter.WriteOffline: evidence path is required")
	}

	if err := os.MkdirAll(r.cfg.EvidencePath, 0755); err != nil {
		return fmt.Errorf("report.Reporter.WriteOffline: mkdir: %w", err)
	}

	path := filepath.Join(r.cfg.EvidencePath, "evidence.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("report.Reporter.WriteOffline: open: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, entry := range entries {
		if err := enc.Encode(entry); err != nil {
			return fmt.Errorf("report.Reporter.WriteOffline: encode: %w", err)
		}
	}

	return nil
}

// Report writes evidence locally and optionally uploads to the Evidra API.
// Online upload failures are logged but do not cause Report to return an error.
func (r *Reporter) Report(entries []EvidenceEntry) error {
	if err := r.WriteOffline(entries); err != nil {
		return err
	}

	if r.cfg.IsOnline() {
		// Online reporting is best-effort in v1.
		// TODO: implement HTTP upload via samebits.com/evidra/pkg/client
		// when the public API surface stabilizes.
		_ = r.uploadBestEffort(entries)
	}

	return nil
}

func (r *Reporter) uploadBestEffort(entries []EvidenceEntry) error {
	return r.uploadBenchmarkRun(entries)
}

func (r *Reporter) uploadBenchmarkRun(entries []EvidenceEntry) error {
	req, err := buildBenchmarkRunRequest(entries)
	if err != nil {
		return err
	}
	client := evidraclient.New(evidraclient.Config{
		URL:    r.cfg.EvidraURL,
		APIKey: r.cfg.EvidraAPIKey,
	})
	if _, err := client.SubmitBenchmarkRun(context.Background(), req); err != nil {
		return fmt.Errorf("report.Reporter.uploadBenchmarkRun: %w", err)
	}
	return nil
}

func buildBenchmarkRunRequest(entries []EvidenceEntry) (evidraclient.BenchmarkRunRequest, error) {
	metadata := map[string]string{
		"source": "infra-bench",
	}
	results := make([]evidraclient.BenchmarkResult, 0, len(entries))
	allPassed := true

	for _, entry := range entries {
		if !entry.Passed {
			allPassed = false
		}
		for key, value := range entry.Metadata {
			metadata[key] = value
		}
		if entry.Actor != "" {
			metadata["actor"] = entry.Actor
		}
		if entry.Adapter != "" {
			metadata["adapter"] = entry.Adapter
		}

		details, err := json.Marshal(map[string]any{
			"id":          entry.ID,
			"actor":       entry.Actor,
			"adapter":     entry.Adapter,
			"exit_code":   entry.ExitCode,
			"duration_ns": entry.Duration,
			"timestamp":   entry.Timestamp,
			"metadata":    entry.Metadata,
		})
		if err != nil {
			return evidraclient.BenchmarkRunRequest{}, fmt.Errorf("report.buildBenchmarkRunRequest: marshal details: %w", err)
		}
		results = append(results, evidraclient.BenchmarkResult{
			CaseID:         entry.ScenarioID,
			ExpectedSignal: "pass",
			ActualSignal:   benchmarkActualSignal(entry.Passed),
			Passed:         entry.Passed,
			Details:        details,
		})
	}

	rawMetadata, err := json.Marshal(metadata)
	if err != nil {
		return evidraclient.BenchmarkRunRequest{}, fmt.Errorf("report.buildBenchmarkRunRequest: marshal metadata: %w", err)
	}

	band := "fail"
	if allPassed {
		band = "pass"
	}

	return evidraclient.BenchmarkRunRequest{
		Suite:    "infra-bench",
		Band:     band,
		Metadata: rawMetadata,
		Results:  results,
	}, nil
}

func benchmarkActualSignal(passed bool) string {
	if passed {
		return "pass"
	}
	return "fail"
}
