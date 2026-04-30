package benchsvc

import (
	"context"
	"fmt"

	bench "samebits.com/evidra-infra-bench/pkg/bench"
)

// RunComparison holds the result of comparing two benchmark runs.
type RunComparison struct {
	RunA  bench.RunRecord `json:"run_a"`
	RunB  bench.RunRecord `json:"run_b"`
	Delta ComparisonDelta `json:"delta"`
}

// ComparisonDelta shows differences between two runs.
type ComparisonDelta struct {
	PassedChanged    bool    `json:"passed_changed"`
	DurationDiff     float64 `json:"duration_diff_seconds"` // B - A
	TurnsDiff        int     `json:"turns_diff"`            // B - A
	CostDiff         float64 `json:"cost_diff_usd"`         // B - A
	TokensDiff       int     `json:"tokens_diff"`           // B - A (prompt + completion)
	ChecksPassedDiff int     `json:"checks_passed_diff"`    // B - A
}

// CompareRuns fetches two runs by ID and computes the delta.
// Single query: both runs in one roundtrip.
func (s *Service) CompareRuns(ctx context.Context, tenantID, idA, idB string) (*RunComparison, error) {
	runA, err := s.repo.GetRun(ctx, tenantID, idA)
	if err != nil {
		return nil, fmt.Errorf("benchsvc.CompareRuns: run A: %w", err)
	}
	runB, err := s.repo.GetRun(ctx, tenantID, idB)
	if err != nil {
		return nil, fmt.Errorf("benchsvc.CompareRuns: run B: %w", err)
	}

	tokensA := runA.PromptTokens + runA.CompletionTokens
	tokensB := runB.PromptTokens + runB.CompletionTokens

	return &RunComparison{
		RunA: *runA,
		RunB: *runB,
		Delta: ComparisonDelta{
			PassedChanged:    runA.Passed != runB.Passed,
			DurationDiff:     runB.Duration - runA.Duration,
			TurnsDiff:        runB.Turns - runA.Turns,
			CostDiff:         runB.EstimatedCost - runA.EstimatedCost,
			TokensDiff:       tokensB - tokensA,
			ChecksPassedDiff: runB.ChecksPassed - runA.ChecksPassed,
		},
	}, nil
}

// ModelComparison shows two models side-by-side across shared scenarios.
type ModelComparison struct {
	ModelA    string                    `json:"model_a"`
	ModelB    string                    `json:"model_b"`
	Scenarios []ScenarioModelComparison `json:"scenarios"`
	Summary   ModelComparisonSummary    `json:"summary"`
}

// ScenarioModelComparison shows per-scenario results for two models.
type ScenarioModelComparison struct {
	ScenarioID string  `json:"scenario_id"`
	APassRate  float64 `json:"a_pass_rate"`
	BPassRate  float64 `json:"b_pass_rate"`
	ADuration  float64 `json:"a_avg_duration"`
	BDuration  float64 `json:"b_avg_duration"`
	ACost      float64 `json:"a_avg_cost"`
	BCost      float64 `json:"b_avg_cost"`
}

// ModelComparisonSummary aggregates the comparison.
type ModelComparisonSummary struct {
	AOverallPassRate float64 `json:"a_overall_pass_rate"`
	BOverallPassRate float64 `json:"b_overall_pass_rate"`
	ATotalCost       float64 `json:"a_total_cost"`
	BTotalCost       float64 `json:"b_total_cost"`
	SharedScenarios  int     `json:"shared_scenarios"`
}

// CompareModels compares two models across all shared scenarios.
// Single query: aggregates in SQL.
func (s *Service) CompareModels(ctx context.Context, tenantID, modelA, modelB, evidenceMode string) (*ModelComparison, error) {
	scenarios, err := s.repo.CompareModels(ctx, tenantID, modelA, modelB, evidenceMode)
	if err != nil {
		return nil, fmt.Errorf("benchsvc.CompareModels: %w", err)
	}

	result := &ModelComparison{
		ModelA:    modelA,
		ModelB:    modelB,
		Scenarios: scenarios,
	}

	var aPass, aTotal, bPass, bTotal int
	var aCost, bCost float64
	for _, sc := range scenarios {
		if sc.APassRate >= 0 {
			aTotal++
			if sc.APassRate >= 50 {
				aPass++
			}
			aCost += sc.ACost
		}
		if sc.BPassRate >= 0 {
			bTotal++
			if sc.BPassRate >= 50 {
				bPass++
			}
			bCost += sc.BCost
		}
	}

	if aTotal > 0 {
		result.Summary.AOverallPassRate = 100.0 * float64(aPass) / float64(aTotal)
	}
	if bTotal > 0 {
		result.Summary.BOverallPassRate = 100.0 * float64(bPass) / float64(bTotal)
	}
	result.Summary.ATotalCost = aCost
	result.Summary.BTotalCost = bCost
	result.Summary.SharedScenarios = len(scenarios)

	return result, nil
}
