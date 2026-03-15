package skilldelta

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPairs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	first := PairResult{ScenarioID: "zeta", Model: "sonnet", Repeat: 2}
	second := PairResult{ScenarioID: "alpha", Model: "haiku", Repeat: 1}

	if err := WritePairJSON(filepath.Join(root, "cases", "zeta", "sonnet", "repeat-2", "pair.json"), first); err != nil {
		t.Fatalf("WritePairJSON(first): %v", err)
	}
	if err := WritePairJSON(filepath.Join(root, "cases", "alpha", "haiku", "repeat-1", "pair.json"), second); err != nil {
		t.Fatalf("WritePairJSON(second): %v", err)
	}

	pairs, err := LoadPairs(root)
	if err != nil {
		t.Fatalf("LoadPairs: %v", err)
	}
	if len(pairs) != 2 {
		t.Fatalf("len(pairs) = %d", len(pairs))
	}
	if pairs[0].ScenarioID != "alpha" {
		t.Fatalf("pairs[0].ScenarioID = %q", pairs[0].ScenarioID)
	}
}

func TestRenderMarkdown(t *testing.T) {
	t.Parallel()

	benchmark := BuildBenchmark(BenchmarkMetadata{
		Suite:       "skill-delta",
		GeneratedAt: "2026-03-15T17:20:00Z",
	}, []PairResult{
		{
			ScenarioID: "broken-deployment",
			Model:      "sonnet",
			Repeat:     1,
			WithoutSkill: RunSnapshot{
				Passed:           false,
				DurationSeconds:  10,
				TotalTokens:      1250,
				EstimatedCostUSD: 0.015,
				Protocol: ProtocolMetrics{
					ComplianceRatePct: 0,
				},
			},
			WithSkill: RunSnapshot{
				Passed:           true,
				DurationSeconds:  14,
				TotalTokens:      1600,
				EstimatedCostUSD: 0.021,
				Protocol: ProtocolMetrics{
					ComplianceRatePct: 100,
				},
				Scorecard: ScorecardMetrics{
					Available: true,
					Score:     floatPtr(96.5),
				},
			},
			ComplianceDeltaPct:   100,
			CostDeltaUSD:         0.006,
			DurationDeltaSeconds: 4,
			ScoreDelta:           96.5,
			TokenDelta: TokenDelta{
				TotalTokens: 350,
			},
		},
	})

	got := RenderMarkdown(benchmark)
	want, err := os.ReadFile(filepath.Join("testdata", "benchmark.md.golden"))
	if err != nil {
		t.Fatalf("ReadFile(golden): %v", err)
	}
	if got != string(want) {
		t.Fatalf("RenderMarkdown() mismatch\n--- got ---\n%s\n--- want ---\n%s", got, string(want))
	}
}

func TestWriteBenchmarkJSON(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "benchmark.json")
	benchmark := Benchmark{Metadata: BenchmarkMetadata{Suite: "skill-delta"}}

	if err := WriteBenchmarkJSON(path, benchmark); err != nil {
		t.Fatalf("WriteBenchmarkJSON: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), `"suite": "skill-delta"`) {
		t.Fatalf("benchmark.json missing suite: %s", string(data))
	}
}

func floatPtr(v float64) *float64 {
	return &v
}
