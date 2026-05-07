package skilldelta

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"samebits.com/evidra-infra-bench/pkg/verifier"
)

func TestBuildPairResultFromRuns(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withoutDir := filepath.Join(root, "cases", "broken-deployment", "sonnet", "repeat-1", "without_skill")
	withDir := filepath.Join(root, "cases", "broken-deployment", "sonnet", "repeat-1", "with_skill")

	withoutEvidenceDir := filepath.Join(withoutDir, "evidence")
	withEvidenceDir := filepath.Join(withDir, "evidence")

	writeRunFixture(t, withoutDir, runFixture{
		ScenarioID: "broken-deployment",
		Passed:     false,
		ExitCode:   1,
		StartTime:  "2026-03-15T17:10:00Z",
		EndTime:    "2026-03-15T17:10:10Z",
		Metadata: map[string]string{
			"model":               "sonnet",
			"provider":            "claude",
			"turns":               "4",
			"prompt_tokens":       "1000",
			"completion_tokens":   "250",
			"estimated_cost":      "0.015",
			"evidence_dir":        withoutEvidenceDir,
			"contract_version":    "v1.0.1",
			"prompt_version":      "v1.0.1",
			"skill_version":       "1.0.1",
			"infra_bench_version": "abc123",
		},
		Checks: verifier.VerifyResult{
			Passed: false,
			Checks: []verifier.CheckResult{
				{Name: "deployment-ready", Type: "deployment-ready", Verdict: verifier.VerdictFail},
				{Name: "protocol/prescribe-count-min", Type: "protocol", Verdict: verifier.VerdictFail},
				{Name: "protocol/report-count-min", Type: "protocol", Verdict: verifier.VerdictFail},
			},
		},
	})
	writeEvidenceFixture(t, withoutEvidenceDir)

	writeRunFixture(t, withDir, runFixture{
		ScenarioID: "broken-deployment",
		Passed:     true,
		ExitCode:   0,
		StartTime:  "2026-03-15T17:11:00Z",
		EndTime:    "2026-03-15T17:11:14Z",
		Metadata: map[string]string{
			"model":               "sonnet",
			"provider":            "claude",
			"turns":               "6",
			"prompt_tokens":       "1300",
			"completion_tokens":   "300",
			"estimated_cost":      "0.021",
			"evidence_dir":        withEvidenceDir,
			"contract_version":    "v1.0.1",
			"prompt_version":      "v1.0.1",
			"skill_version":       "1.0.1",
			"infra_bench_version": "abc123",
		},
		Checks: verifier.VerifyResult{
			Passed: true,
			Checks: []verifier.CheckResult{
				{Name: "deployment-ready", Type: "deployment-ready", Verdict: verifier.VerdictPass},
				{Name: "protocol/prescribe-count-min", Type: "protocol", Verdict: verifier.VerdictPass},
				{Name: "protocol/report-count-min", Type: "protocol", Verdict: verifier.VerdictPass},
			},
		},
	})
	writeEvidenceFixture(t, withEvidenceDir,
		jsonLine(t, map[string]any{
			"type": "prescribe",
			"payload": map[string]any{
				"prescription_id": "pres-1",
			},
		}),
		jsonLine(t, map[string]any{
			"type": "prescribe",
			"payload": map[string]any{
				"prescription_id": "pres-2",
			},
		}),
		jsonLine(t, map[string]any{
			"type": "report",
			"payload": map[string]any{
				"prescription_id": "pres-1",
				"verdict":         "success",
			},
		}),
		jsonLine(t, map[string]any{
			"type": "report",
			"payload": map[string]any{
				"prescription_id": "pres-2",
				"verdict":         "declined",
			},
		}),
		jsonLine(t, map[string]any{
			"type": "signal",
			"payload": map[string]any{
				"signal_name": "repair_loop",
			},
		}),
	)
	writeScorecardFixture(t, filepath.Join(withDir, "evidence", "scorecard.json"), map[string]any{
		"score": 96.5,
		"band":  "good",
		"signal_summary": map[string]any{
			"repair_loop": map[string]any{
				"detected": true,
				"count":    1,
			},
			"retry_loop": map[string]any{
				"detected": false,
				"count":    0,
			},
		},
	})

	got, err := BuildPairResult(withoutDir, withDir)
	if err != nil {
		t.Fatalf("BuildPairResult: %v", err)
	}

	if got.ScenarioID != "broken-deployment" {
		t.Fatalf("ScenarioID = %q", got.ScenarioID)
	}
	if got.Model != "sonnet" {
		t.Fatalf("Model = %q", got.Model)
	}
	if got.Provider != "claude" {
		t.Fatalf("Provider = %q", got.Provider)
	}
	if got.Repeat != 1 {
		t.Fatalf("Repeat = %d", got.Repeat)
	}
	if got.VerdictDelta != "improved" {
		t.Fatalf("VerdictDelta = %q", got.VerdictDelta)
	}
	if got.TokenDelta.TotalTokens != 350 {
		t.Fatalf("TokenDelta.TotalTokens = %d", got.TokenDelta.TotalTokens)
	}
	if got.DurationDeltaSeconds != 4 {
		t.Fatalf("DurationDeltaSeconds = %v", got.DurationDeltaSeconds)
	}
	if got.ComplianceDeltaPct != 100 {
		t.Fatalf("ComplianceDeltaPct = %v", got.ComplianceDeltaPct)
	}
	if got.WithoutSkill.Protocol.ChecksTotal != 2 {
		t.Fatalf("WithoutSkill.Protocol.ChecksTotal = %d", got.WithoutSkill.Protocol.ChecksTotal)
	}
	if got.WithSkill.Protocol.PrescribeCount != 2 {
		t.Fatalf("WithSkill.Protocol.PrescribeCount = %d", got.WithSkill.Protocol.PrescribeCount)
	}
	if got.WithSkill.Protocol.ReportCount != 2 {
		t.Fatalf("WithSkill.Protocol.ReportCount = %d", got.WithSkill.Protocol.ReportCount)
	}
	if got.WithSkill.Protocol.DeclinedCount != 1 {
		t.Fatalf("WithSkill.Protocol.DeclinedCount = %d", got.WithSkill.Protocol.DeclinedCount)
	}
	if got.WithSkill.Protocol.VerdictCoveragePct != 100 {
		t.Fatalf("WithSkill.Protocol.VerdictCoveragePct = %v", got.WithSkill.Protocol.VerdictCoveragePct)
	}
	if got.WithSkill.Scorecard.Score == nil || *got.WithSkill.Scorecard.Score != 96.5 {
		t.Fatalf("WithSkill.Scorecard.Score = %v", got.WithSkill.Scorecard.Score)
	}
	if got.WithSkill.Scorecard.Band != "good" {
		t.Fatalf("WithSkill.Scorecard.Band = %q", got.WithSkill.Scorecard.Band)
	}
	if got.ScoreDelta != 96.5 {
		t.Fatalf("ScoreDelta = %v", got.ScoreDelta)
	}
	if got.Paths.WithSkillScorecard == "" {
		t.Fatal("WithSkillScorecard path missing")
	}
}

type runFixture struct {
	ScenarioID string
	Passed     bool
	ExitCode   int
	StartTime  string
	EndTime    string
	Metadata   map[string]string
	Checks     verifier.VerifyResult
}

func writeRunFixture(t *testing.T, runDir string, fixture runFixture) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(runDir, "evidence"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "transcript.txt"), []byte("transcript"), 0o644); err != nil {
		t.Fatal(err)
	}

	checksJSON, err := json.Marshal(fixture.Checks)
	if err != nil {
		t.Fatal(err)
	}

	runJSON, err := json.Marshal(map[string]any{
		"scenario_id": fixture.ScenarioID,
		"passed":      fixture.Passed,
		"exit_code":   fixture.ExitCode,
		"start_time":  fixture.StartTime,
		"end_time":    fixture.EndTime,
		"checks":      json.RawMessage(checksJSON),
		"metadata":    fixture.Metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "run.json"), runJSON, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeEvidenceFixture(t *testing.T, evidenceDir string, lines ...string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(evidenceDir, "segments"), 0o755); err != nil {
		t.Fatal(err)
	}
	if len(lines) == 0 {
		lines = []string{}
	}
	data := []byte{}
	for _, line := range lines {
		data = append(data, []byte(line+"\n")...)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "segments", "part-0001.jsonl"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeScorecardFixture(t *testing.T, path string, body map[string]any) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func jsonLine(t *testing.T, v map[string]any) string {
	t.Helper()

	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
