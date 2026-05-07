package skilldelta

import (
	"encoding/json"
	"testing"
)

func TestBenchmarkJSONShape(t *testing.T) {
	t.Parallel()

	score := 94.5
	doc := Benchmark{
		Metadata: BenchmarkMetadata{
			Suite:             "skill-delta",
			GeneratedAt:       "2026-03-15T17:00:00Z",
			Repeats:           3,
			Provider:          "claude",
			Scenarios:         []string{"broken-deployment"},
			Models:            []string{"sonnet"},
			NoSkillPrompt:     "prompts/no-skill.md",
			WithSkillPrompt:   "skills/runtime-contract.md",
			InfraBenchVersion: "875c7be",
			ContractVersion:   "v1.0.1",
			PromptVersion:     "v1.0.1",
			SkillVersion:      "1.0.1",
		},
		Pairs: []PairResult{
			{
				ScenarioID: "broken-deployment",
				Model:      "sonnet",
				Provider:   "claude",
				Repeat:     1,
				WithoutSkill: RunSnapshot{
					Passed:           false,
					DurationSeconds:  12.5,
					Turns:            6,
					PromptTokens:     1000,
					CompletionTokens: 250,
					EstimatedCostUSD: 0.015,
					ChecksPassed:     0,
					ChecksTotal:      1,
					Protocol: ProtocolMetrics{
						PrescribeCount:        0,
						ReportCount:           0,
						ChecksPassed:          0,
						ChecksTotal:           2,
						ComplianceRatePct:     0,
						VerdictCoveragePct:    0,
						OrphanedPrescriptions: 0,
					},
					Scorecard: ScorecardMetrics{
						Available: false,
					},
				},
				WithSkill: RunSnapshot{
					Passed:           true,
					DurationSeconds:  14.0,
					Turns:            7,
					PromptTokens:     1300,
					CompletionTokens: 300,
					EstimatedCostUSD: 0.021,
					ChecksPassed:     1,
					ChecksTotal:      1,
					Protocol: ProtocolMetrics{
						PrescribeCount:        2,
						ReportCount:           2,
						ChecksPassed:          2,
						ChecksTotal:           2,
						ComplianceRatePct:     100,
						VerdictCoveragePct:    100,
						OrphanedPrescriptions: 0,
					},
					Scorecard: ScorecardMetrics{
						Available: true,
						Score:     &score,
						Band:      "good",
						Signals:   []string{"repair_loop"},
						SignalCounts: map[string]int{
							"repair_loop": 1,
						},
					},
				},
				VerdictDelta:         "improved",
				DurationDeltaSeconds: 1.5,
				CostDeltaUSD:         0.006,
				ComplianceDeltaPct:   100,
				ScoreDelta:           94.5,
				TokenDelta: TokenDelta{
					PromptTokens:     300,
					CompletionTokens: 50,
					TotalTokens:      350,
				},
				Paths: PairPaths{
					WithoutSkillRunDir: "cases/broken-deployment/sonnet/repeat-1/without_skill",
					WithSkillRunDir:    "cases/broken-deployment/sonnet/repeat-1/with_skill",
				},
			},
		},
		Summary: BenchmarkSummary{
			PairCount: 1,
			WithoutSkill: ConfigurationSummary{
				PassRatePct: NumericSummary{Mean: 0, Stddev: 0, Min: 0, Max: 0},
			},
			WithSkill: ConfigurationSummary{
				PassRatePct: NumericSummary{Mean: 100, Stddev: 0, Min: 100, Max: 100},
			},
			Delta: DeltaSummary{
				PassRatePct:   100,
				CompliancePct: 100,
				TotalTokens:   350,
				Score:         94.5,
			},
		},
	}

	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if _, ok := got["metadata"]; !ok {
		t.Fatal("benchmark missing metadata")
	}
	if _, ok := got["pairs"]; !ok {
		t.Fatal("benchmark missing pairs")
	}
	if _, ok := got["summary"]; !ok {
		t.Fatal("benchmark missing summary")
	}

	metadata := got["metadata"].(map[string]any)
	if metadata["suite"] != "skill-delta" {
		t.Fatalf("metadata.suite = %v", metadata["suite"])
	}
	if metadata["skill_version"] != "1.0.1" {
		t.Fatalf("metadata.skill_version = %v", metadata["skill_version"])
	}

	pairs := got["pairs"].([]any)
	if len(pairs) != 1 {
		t.Fatalf("len(pairs) = %d, want 1", len(pairs))
	}
	pair := pairs[0].(map[string]any)
	if pair["scenario_id"] != "broken-deployment" {
		t.Fatalf("pair.scenario_id = %v", pair["scenario_id"])
	}
	if pair["verdict_delta"] != "improved" {
		t.Fatalf("pair.verdict_delta = %v", pair["verdict_delta"])
	}
	if _, ok := pair["without_skill"].(map[string]any)["protocol"]; !ok {
		t.Fatal("pair.without_skill.protocol missing")
	}
	if _, ok := pair["with_skill"].(map[string]any)["scorecard"]; !ok {
		t.Fatal("pair.with_skill.scorecard missing")
	}

	summary := got["summary"].(map[string]any)
	if summary["pair_count"] != float64(1) {
		t.Fatalf("summary.pair_count = %v", summary["pair_count"])
	}
	delta := summary["delta"].(map[string]any)
	if delta["pass_rate_pct"] != float64(100) {
		t.Fatalf("summary.delta.pass_rate_pct = %v", delta["pass_rate_pct"])
	}
	if delta["total_tokens"] != float64(350) {
		t.Fatalf("summary.delta.total_tokens = %v", delta["total_tokens"])
	}
}
