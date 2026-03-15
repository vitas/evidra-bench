package signalaudit_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"samebits.com/evidra-infra-bench/pkg/signalaudit"
)

func TestLoadRun_UsesScorecardWhenPresent(t *testing.T) {
	runDir := writeAuditRunFixture(t, auditRunFixture{
		ScenarioID: "broken-deployment",
		Metadata: map[string]string{
			"model":    "sonnet",
			"provider": "claude",
		},
		ScorecardSignals: map[string]any{
			"retry_loop": map[string]any{"count": 1},
		},
	})

	got, err := signalaudit.LoadRun(runDir)
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if got.ScenarioID != "broken-deployment" {
		t.Fatalf("scenario_id = %q, want broken-deployment", got.ScenarioID)
	}
	if got.Model != "sonnet" {
		t.Fatalf("model = %q, want sonnet", got.Model)
	}
	if got.Provider != "claude" {
		t.Fatalf("provider = %q, want claude", got.Provider)
	}
	if got.SignalCounts["retry_loop"] != 1 {
		t.Fatalf("retry_loop count = %d, want 1", got.SignalCounts["retry_loop"])
	}
	if got.SignalCounts["blast_radius"] != 0 {
		t.Fatalf("blast_radius count = %d, want 0", got.SignalCounts["blast_radius"])
	}
}

func TestLoadRun_FallsBackToEvidenceSignals(t *testing.T) {
	runDir := writeAuditRunFixture(t, auditRunFixture{
		ScenarioID: "networkpolicy-blocking",
		Metadata: map[string]string{
			"model":        "sonnet",
			"provider":     "claude",
			"evidence_dir": filepath.Join(t.TempDir(), "unused"),
		},
		EvidenceSignals: []string{"blast_radius", "blast_radius", "new_scope"},
	})

	got, err := signalaudit.LoadRun(runDir)
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if got.SignalCounts["blast_radius"] != 2 {
		t.Fatalf("blast_radius count = %d, want 2", got.SignalCounts["blast_radius"])
	}
	if got.SignalCounts["new_scope"] != 1 {
		t.Fatalf("new_scope count = %d, want 1", got.SignalCounts["new_scope"])
	}
}

type auditRunFixture struct {
	ScenarioID      string
	Metadata        map[string]string
	ScorecardSignals map[string]any
	EvidenceSignals []string
}

func writeAuditRunFixture(t *testing.T, fixture auditRunFixture) string {
	t.Helper()

	runDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(runDir, "evidra", "segments"), 0755); err != nil {
		t.Fatalf("mkdir evidra segments: %v", err)
	}

	metadata := map[string]string{}
	for key, value := range fixture.Metadata {
		metadata[key] = value
	}
	if _, ok := metadata["evidence_dir"]; !ok {
		metadata["evidence_dir"] = filepath.Join(runDir, "evidra")
	}

	runJSON := map[string]any{
		"scenario_id": fixture.ScenarioID,
		"passed":      true,
		"start_time":  time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
		"end_time":    time.Date(2026, 3, 15, 12, 1, 0, 0, time.UTC),
		"metadata":    metadata,
	}
	writeJSONFixture(t, filepath.Join(runDir, "run.json"), runJSON)

	if len(fixture.ScorecardSignals) > 0 {
		writeJSONFixture(t, filepath.Join(runDir, "evidra", "scorecard.json"), map[string]any{
			"score":          88.5,
			"band":           "good",
			"signal_summary": fixture.ScorecardSignals,
		})
	}

	if len(fixture.EvidenceSignals) > 0 {
		path := filepath.Join(runDir, "evidra", "segments", "0001.jsonl")
		file, err := os.Create(path)
		if err != nil {
			t.Fatalf("create evidence fixture: %v", err)
		}
		for _, signal := range fixture.EvidenceSignals {
			entry := map[string]any{
				"entry_id":   "signal-" + signal,
				"type":       "signal",
				"timestamp":  "2026-03-15T12:00:00Z",
				"actor":      map[string]any{"id": "evidra"},
				"payload":    map[string]any{"signal_name": signal},
			}
			data, err := json.Marshal(entry)
			if err != nil {
				t.Fatalf("marshal evidence entry: %v", err)
			}
			if _, err := file.Write(append(data, '\n')); err != nil {
				t.Fatalf("write evidence entry: %v", err)
			}
		}
		if err := file.Close(); err != nil {
			t.Fatalf("close evidence fixture: %v", err)
		}
	}

	return runDir
}

func writeJSONFixture(t *testing.T, path string, value any) {
	t.Helper()

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal json fixture: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir fixture dir: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatalf("write json fixture: %v", err)
	}
}
