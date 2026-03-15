package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"samebits.com/evidra-infra-bench/pkg/skilldelta"
)

func TestWriteSkillDeltaHTML(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outputPath := filepath.Join(root, "benchmark.html")

	benchmark := skilldelta.BuildBenchmark(skilldelta.BenchmarkMetadata{
		Suite:       "skill-delta",
		GeneratedAt: "2026-03-15T18:00:00Z",
	}, []skilldelta.PairResult{
		{
			ScenarioID: "broken-deployment",
			Model:      "sonnet",
			Repeat:     1,
			WithoutSkill: skilldelta.RunSnapshot{
				Passed: false,
			},
			WithSkill: skilldelta.RunSnapshot{
				Passed: true,
				Scorecard: skilldelta.ScorecardMetrics{
					Available: true,
					Band:      "good",
					Score:     floatPtr(96.5),
					Signals:   []string{"repair_loop"},
				},
			},
			ComplianceDeltaPct:   100,
			CostDeltaUSD:         0.006,
			DurationDeltaSeconds: 4,
			ScoreDelta:           96.5,
			TokenDelta: skilldelta.TokenDelta{
				TotalTokens: 350,
			},
			Paths: skilldelta.PairPaths{
				WithoutSkillRunDir: filepath.Join(root, "cases", "broken-deployment", "sonnet", "repeat-1", "without_skill"),
				WithSkillRunDir:    filepath.Join(root, "cases", "broken-deployment", "sonnet", "repeat-1", "with_skill"),
			},
		},
	})

	if err := WriteSkillDeltaHTML(outputPath, benchmark); err != nil {
		t.Fatalf("WriteSkillDeltaHTML: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	html := string(data)
	for _, want := range []string{
		"Skill Delta Benchmark",
		"Pass rate delta",
		"broken-deployment",
		"repair_loop",
		"cases/broken-deployment/sonnet/repeat-1/with_skill",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing %q\n%s", want, html)
		}
	}
}

func floatPtr(v float64) *float64 {
	return &v
}
