package skilldelta

import (
	"math"
	"testing"
)

func TestAggregateBenchmark(t *testing.T) {
	t.Parallel()

	scoreA := 96.5
	scoreBWithout := 80.0
	scoreBWith := 90.0
	summary := Aggregate([]PairResult{
		{
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
					Score:     &scoreA,
				},
			},
		},
		{
			WithoutSkill: RunSnapshot{
				Passed:           true,
				DurationSeconds:  20,
				TotalTokens:      1000,
				EstimatedCostUSD: 0.020,
				Protocol: ProtocolMetrics{
					ComplianceRatePct: 50,
				},
				Scorecard: ScorecardMetrics{
					Available: true,
					Score:     &scoreBWithout,
				},
			},
			WithSkill: RunSnapshot{
				Passed:           true,
				DurationSeconds:  18,
				TotalTokens:      1200,
				EstimatedCostUSD: 0.025,
				Protocol: ProtocolMetrics{
					ComplianceRatePct: 100,
				},
				Scorecard: ScorecardMetrics{
					Available: true,
					Score:     &scoreBWith,
				},
			},
		},
	})

	if summary.PairCount != 2 {
		t.Fatalf("PairCount = %d", summary.PairCount)
	}
	if summary.WithoutSkill.PassRatePct.Mean != 50 {
		t.Fatalf("WithoutSkill.PassRatePct.Mean = %v", summary.WithoutSkill.PassRatePct.Mean)
	}
	if summary.WithSkill.PassRatePct.Mean != 100 {
		t.Fatalf("WithSkill.PassRatePct.Mean = %v", summary.WithSkill.PassRatePct.Mean)
	}
	if summary.WithoutSkill.CompliancePct.Mean != 25 {
		t.Fatalf("WithoutSkill.CompliancePct.Mean = %v", summary.WithoutSkill.CompliancePct.Mean)
	}
	if summary.WithSkill.CompliancePct.Mean != 100 {
		t.Fatalf("WithSkill.CompliancePct.Mean = %v", summary.WithSkill.CompliancePct.Mean)
	}
	if summary.WithoutSkill.DurationSeconds.Mean != 15 {
		t.Fatalf("WithoutSkill.DurationSeconds.Mean = %v", summary.WithoutSkill.DurationSeconds.Mean)
	}
	if summary.WithSkill.TotalTokens.Mean != 1400 {
		t.Fatalf("WithSkill.TotalTokens.Mean = %v", summary.WithSkill.TotalTokens.Mean)
	}
	if !almostEqual(summary.WithoutSkill.DurationSeconds.Stddev, 7.0711, 0.0001) {
		t.Fatalf("WithoutSkill.DurationSeconds.Stddev = %v", summary.WithoutSkill.DurationSeconds.Stddev)
	}
	if !almostEqual(summary.Delta.PassRatePct, 50, 0.0001) {
		t.Fatalf("Delta.PassRatePct = %v", summary.Delta.PassRatePct)
	}
	if !almostEqual(summary.Delta.CompliancePct, 75, 0.0001) {
		t.Fatalf("Delta.CompliancePct = %v", summary.Delta.CompliancePct)
	}
	if !almostEqual(summary.Delta.TotalTokens, 275, 0.0001) {
		t.Fatalf("Delta.TotalTokens = %v", summary.Delta.TotalTokens)
	}
	if !almostEqual(summary.Delta.Score, 13.25, 0.0001) {
		t.Fatalf("Delta.Score = %v", summary.Delta.Score)
	}
}

func TestAggregateBenchmark_Empty(t *testing.T) {
	t.Parallel()

	summary := Aggregate(nil)
	if summary.PairCount != 0 {
		t.Fatalf("PairCount = %d", summary.PairCount)
	}
	if summary.Delta.PassRatePct != 0 {
		t.Fatalf("Delta.PassRatePct = %v", summary.Delta.PassRatePct)
	}
}

func TestAggregateBenchmark_SinglePairHasZeroStddev(t *testing.T) {
	t.Parallel()

	summary := Aggregate([]PairResult{
		{
			WithoutSkill: RunSnapshot{
				Passed:           true,
				DurationSeconds:  11,
				TotalTokens:      100,
				EstimatedCostUSD: 0.01,
				Protocol: ProtocolMetrics{
					ComplianceRatePct: 100,
				},
			},
			WithSkill: RunSnapshot{
				Passed:           true,
				DurationSeconds:  12,
				TotalTokens:      140,
				EstimatedCostUSD: 0.012,
				Protocol: ProtocolMetrics{
					ComplianceRatePct: 100,
				},
			},
		},
	})

	if summary.WithoutSkill.DurationSeconds.Stddev != 0 {
		t.Fatalf("WithoutSkill.DurationSeconds.Stddev = %v", summary.WithoutSkill.DurationSeconds.Stddev)
	}
	if summary.WithSkill.TotalTokens.Stddev != 0 {
		t.Fatalf("WithSkill.TotalTokens.Stddev = %v", summary.WithSkill.TotalTokens.Stddev)
	}
}

func almostEqual(a, b, eps float64) bool {
	return math.Abs(a-b) <= eps
}
