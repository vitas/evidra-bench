// Package artifact writes local run artifact bundles.
package artifact

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// RunBundle holds all data for a single benchmark run.
type RunBundle struct {
	ScenarioID     string            `json:"scenario_id"`
	Adapter        string            `json:"adapter"`
	StartTime      time.Time         `json:"start_time"`
	EndTime        time.Time         `json:"end_time"`
	ExitCode       int               `json:"exit_code"`
	Passed         bool              `json:"passed"`
	Prompt         string            `json:"prompt,omitempty"`
	Transcript     string            `json:"transcript,omitempty"`
	Stdout         string            `json:"stdout,omitempty"`
	Stderr         string            `json:"stderr,omitempty"`
	ToolCalls      json.RawMessage   `json:"tool_calls,omitempty"`
	Timeline       json.RawMessage   `json:"timeline,omitempty"`
	Checks         json.RawMessage   `json:"checks,omitempty"`
	Autopsy        json.RawMessage   `json:"autopsy,omitempty"`
	RunError       json.RawMessage   `json:"run_error,omitempty"`
	RunEvents      json.RawMessage   `json:"run_events,omitempty"`
	ChaosEnabled   bool              `json:"chaos_enabled,omitempty"`
	ChaosMode      string            `json:"chaos_mode,omitempty"`
	ChaosStepCount int               `json:"chaos_step_count,omitempty"`
	ChaosTimeline  json.RawMessage   `json:"chaos_timeline,omitempty"`
	ChaosLog       string            `json:"-"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// WriteOutput is the result of writing an artifact bundle.
type WriteOutput struct {
	Path string
}

// Writer writes run artifact bundles to a base directory.
type Writer struct {
	BaseDir string
}

// NewWriter creates a Writer that writes bundles under baseDir.
func NewWriter(baseDir string) *Writer {
	return &Writer{BaseDir: baseDir}
}

// Write creates a run artifact directory and writes all bundle files.
func (w *Writer) Write(bundle RunBundle) (*WriteOutput, error) {
	dirName := fmt.Sprintf("%s-%s-%s",
		bundle.StartTime.Format("20060102-150405"),
		bundle.ScenarioID,
		bundle.Adapter,
	)
	runDir := filepath.Join(w.BaseDir, dirName)

	if err := os.MkdirAll(runDir, 0755); err != nil {
		return nil, fmt.Errorf("artifact.Writer.Write: mkdir: %w", err)
	}

	// Create evidence subdirectory for optional local evidence output.
	if err := os.MkdirAll(filepath.Join(runDir, "evidence"), 0755); err != nil {
		return nil, fmt.Errorf("artifact.Writer.Write: mkdir evidence: %w", err)
	}

	// Write run.json metadata.
	runJSON, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("artifact.Writer.Write: marshal run.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "run.json"), runJSON, 0644); err != nil {
		return nil, fmt.Errorf("artifact.Writer.Write: write run.json: %w", err)
	}

	// Write text artifacts.
	textFiles := map[string]string{
		"prompt.txt":     bundle.Prompt,
		"transcript.txt": bundle.Transcript,
		"stdout.txt":     bundle.Stdout,
		"stderr.txt":     bundle.Stderr,
		"chaos.log":      bundle.ChaosLog,
	}
	for name, content := range textFiles {
		if content == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(runDir, name), []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("artifact.Writer.Write: write %s: %w", name, err)
		}
	}

	// Write tool-calls.json if present.
	if len(bundle.ToolCalls) > 0 {
		if err := os.WriteFile(filepath.Join(runDir, "tool-calls.json"), bundle.ToolCalls, 0644); err != nil {
			return nil, fmt.Errorf("artifact.Writer.Write: write tool-calls.json: %w", err)
		}
	}

	// Write timeline.json if present.
	if len(bundle.Timeline) > 0 {
		if err := os.WriteFile(filepath.Join(runDir, "timeline.json"), bundle.Timeline, 0644); err != nil {
			return nil, fmt.Errorf("artifact.Writer.Write: write timeline.json: %w", err)
		}
	}

	// Write verifier.json if present.
	if len(bundle.Checks) > 0 {
		if err := os.WriteFile(filepath.Join(runDir, "verifier.json"), bundle.Checks, 0644); err != nil {
			return nil, fmt.Errorf("artifact.Writer.Write: write verifier.json: %w", err)
		}
	}

	// Write failure-autopsy.json if present.
	if len(bundle.Autopsy) > 0 {
		if err := os.WriteFile(filepath.Join(runDir, "failure-autopsy.json"), bundle.Autopsy, 0644); err != nil {
			return nil, fmt.Errorf("artifact.Writer.Write: write failure-autopsy.json: %w", err)
		}
	}

	// Write run-error.json if present.
	if len(bundle.RunError) > 0 {
		if err := os.WriteFile(filepath.Join(runDir, "run-error.json"), bundle.RunError, 0644); err != nil {
			return nil, fmt.Errorf("artifact.Writer.Write: write run-error.json: %w", err)
		}
	}

	// Write run-events.json if present.
	if len(bundle.RunEvents) > 0 {
		if err := os.WriteFile(filepath.Join(runDir, "run-events.json"), bundle.RunEvents, 0644); err != nil {
			return nil, fmt.Errorf("artifact.Writer.Write: write run-events.json: %w", err)
		}
	}

	// Write chaos.json if present.
	if len(bundle.ChaosTimeline) > 0 {
		if err := os.WriteFile(filepath.Join(runDir, "chaos.json"), bundle.ChaosTimeline, 0644); err != nil {
			return nil, fmt.Errorf("artifact.Writer.Write: write chaos.json: %w", err)
		}
	}

	return &WriteOutput{Path: runDir}, nil
}
