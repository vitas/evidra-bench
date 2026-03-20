// Package report provides offline Evidra evidence writing for benchmark runs.
// Online reporting is handled by harness.ReportToEvidra (POST /v1/bench/runs).
package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Config configures the Evidra reporter.
type Config struct {
	EvidencePath string
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

// Reporter writes Evidra evidence to local JSONL files.
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
	defer func() { _ = f.Close() }()

	enc := json.NewEncoder(f)
	for _, entry := range entries {
		if err := enc.Encode(entry); err != nil {
			return fmt.Errorf("report.Reporter.WriteOffline: encode: %w", err)
		}
	}

	return nil
}

// Report writes evidence locally.
func (r *Reporter) Report(entries []EvidenceEntry) error {
	return r.WriteOffline(entries)
}
