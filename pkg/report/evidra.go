// Package report provides optional Evidra reporting for benchmark runs.
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

func (r *Reporter) uploadBestEffort(_ []EvidenceEntry) error {
	// Placeholder for online upload.
	// This will use samebits.com/evidra/pkg/client when ready.
	return nil
}
