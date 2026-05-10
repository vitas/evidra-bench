package benchsvc

import (
	"context"
	"fmt"
	"sort"

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

// ToolServerCompareRequest selects a baseline/native-tools vs MCP server comparison.
type ToolServerCompareRequest struct {
	Model             string
	ToolServer        string
	ToolServerVersion string
	ScenarioIDs       []string
}

// ToolServerComparison shows baseline/native-tools performance beside one MCP server.
type ToolServerComparison struct {
	Model              string                         `json:"model"`
	ToolServer         string                         `json:"tool_server"`
	ToolServerVersion  string                         `json:"tool_server_version,omitempty"`
	ScenarioIDs        []string                       `json:"scenario_ids,omitempty"`
	Baseline           ToolServerAggregate            `json:"baseline"`
	Candidate          ToolServerAggregate            `json:"candidate"`
	Delta              ToolServerMetricDelta          `json:"delta"`
	Scenarios          []ToolServerScenarioComparison `json:"scenarios"`
	ImprovedScenarios  []ToolServerScenarioComparison `json:"improved_scenarios"`
	RegressedScenarios []ToolServerScenarioComparison `json:"regressed_scenarios"`
}

// ToolServerAggregate summarizes a run set.
type ToolServerAggregate struct {
	Runs        int     `json:"runs"`
	Passed      int     `json:"passed"`
	PassRate    float64 `json:"pass_rate"`
	AvgTurns    float64 `json:"avg_turns"`
	AvgTokens   float64 `json:"avg_tokens"`
	AvgCost     float64 `json:"avg_cost_usd"`
	AvgDuration float64 `json:"avg_duration_seconds"`
}

// ToolServerMetricDelta reports candidate minus baseline.
type ToolServerMetricDelta struct {
	PassRateDelta    float64 `json:"pass_rate_delta"`
	AvgTurnsDelta    float64 `json:"avg_turns_delta"`
	AvgTokensDelta   float64 `json:"avg_tokens_delta"`
	AvgCostDelta     float64 `json:"avg_cost_usd_delta"`
	AvgDurationDelta float64 `json:"avg_duration_seconds_delta"`
}

// ToolServerScenarioComparison is a per-scenario baseline vs candidate row.
type ToolServerScenarioComparison struct {
	ScenarioID string                `json:"scenario_id"`
	Baseline   ToolServerAggregate   `json:"baseline"`
	Candidate  ToolServerAggregate   `json:"candidate"`
	Delta      ToolServerMetricDelta `json:"delta"`
}

// CompareToolServer compares direct/native-tool runs to runs using one MCP server.
func (s *Service) CompareToolServer(ctx context.Context, tenantID string, req ToolServerCompareRequest) (*ToolServerComparison, error) {
	baselineRuns, _, err := s.repo.ListRuns(ctx, tenantID, bench.RunFilters{
		Model:        req.Model,
		EvidenceMode: "none",
		ScenarioIDs:  req.ScenarioIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("benchsvc.CompareToolServer: baseline: %w", err)
	}

	candidateRuns, _, err := s.repo.ListRuns(ctx, tenantID, bench.RunFilters{
		Model:             req.Model,
		EvidenceMode:      "mcp",
		ToolServer:        req.ToolServer,
		ToolServerVersion: req.ToolServerVersion,
		ScenarioIDs:       req.ScenarioIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("benchsvc.CompareToolServer: candidate: %w", err)
	}

	baseline := aggregateToolServerRuns(baselineRuns)
	candidate := aggregateToolServerRuns(candidateRuns)
	scenarios := compareToolServerScenarios(baselineRuns, candidateRuns)
	improved, regressed := splitToolServerScenarioDeltas(scenarios)

	return &ToolServerComparison{
		Model:              req.Model,
		ToolServer:         req.ToolServer,
		ToolServerVersion:  req.ToolServerVersion,
		ScenarioIDs:        append([]string(nil), req.ScenarioIDs...),
		Baseline:           baseline,
		Candidate:          candidate,
		Delta:              diffToolServerAggregates(baseline, candidate),
		Scenarios:          scenarios,
		ImprovedScenarios:  improved,
		RegressedScenarios: regressed,
	}, nil
}

func compareToolServerScenarios(baselineRuns, candidateRuns []bench.RunRecord) []ToolServerScenarioComparison {
	baselineByScenario := groupRunsByScenario(baselineRuns)
	candidateByScenario := groupRunsByScenario(candidateRuns)

	idsByScenario := make(map[string]struct{}, len(baselineByScenario)+len(candidateByScenario))
	for scenarioID := range baselineByScenario {
		idsByScenario[scenarioID] = struct{}{}
	}
	for scenarioID := range candidateByScenario {
		idsByScenario[scenarioID] = struct{}{}
	}

	scenarioIDs := make([]string, 0, len(idsByScenario))
	for scenarioID := range idsByScenario {
		scenarioIDs = append(scenarioIDs, scenarioID)
	}
	sort.Strings(scenarioIDs)

	out := make([]ToolServerScenarioComparison, 0, len(scenarioIDs))
	for _, scenarioID := range scenarioIDs {
		baseline := aggregateToolServerRuns(baselineByScenario[scenarioID])
		candidate := aggregateToolServerRuns(candidateByScenario[scenarioID])
		out = append(out, ToolServerScenarioComparison{
			ScenarioID: scenarioID,
			Baseline:   baseline,
			Candidate:  candidate,
			Delta:      diffToolServerAggregates(baseline, candidate),
		})
	}
	return out
}

func groupRunsByScenario(runs []bench.RunRecord) map[string][]bench.RunRecord {
	grouped := make(map[string][]bench.RunRecord)
	for _, run := range runs {
		grouped[run.ScenarioID] = append(grouped[run.ScenarioID], run)
	}
	return grouped
}

func aggregateToolServerRuns(runs []bench.RunRecord) ToolServerAggregate {
	var out ToolServerAggregate
	var turns, tokens int
	var cost, duration float64

	for _, run := range runs {
		out.Runs++
		if run.Passed {
			out.Passed++
		}
		turns += run.Turns
		tokens += run.PromptTokens + run.CompletionTokens
		cost += run.EstimatedCost
		duration += run.Duration
	}

	if out.Runs == 0 {
		return out
	}
	runsCount := float64(out.Runs)
	out.PassRate = 100.0 * float64(out.Passed) / runsCount
	out.AvgTurns = float64(turns) / runsCount
	out.AvgTokens = float64(tokens) / runsCount
	out.AvgCost = cost / runsCount
	out.AvgDuration = duration / runsCount
	return out
}

func diffToolServerAggregates(baseline, candidate ToolServerAggregate) ToolServerMetricDelta {
	return ToolServerMetricDelta{
		PassRateDelta:    candidate.PassRate - baseline.PassRate,
		AvgTurnsDelta:    candidate.AvgTurns - baseline.AvgTurns,
		AvgTokensDelta:   candidate.AvgTokens - baseline.AvgTokens,
		AvgCostDelta:     candidate.AvgCost - baseline.AvgCost,
		AvgDurationDelta: candidate.AvgDuration - baseline.AvgDuration,
	}
}

func splitToolServerScenarioDeltas(scenarios []ToolServerScenarioComparison) ([]ToolServerScenarioComparison, []ToolServerScenarioComparison) {
	improved := make([]ToolServerScenarioComparison, 0)
	regressed := make([]ToolServerScenarioComparison, 0)
	for _, scenario := range scenarios {
		if scenario.Baseline.Runs == 0 || scenario.Candidate.Runs == 0 {
			continue
		}
		if scenario.Delta.PassRateDelta > 0 {
			improved = append(improved, scenario)
		}
		if scenario.Delta.PassRateDelta < 0 {
			regressed = append(regressed, scenario)
		}
	}
	sort.Slice(improved, func(i, j int) bool {
		if improved[i].Delta.PassRateDelta != improved[j].Delta.PassRateDelta {
			return improved[i].Delta.PassRateDelta > improved[j].Delta.PassRateDelta
		}
		return improved[i].ScenarioID < improved[j].ScenarioID
	})
	sort.Slice(regressed, func(i, j int) bool {
		if regressed[i].Delta.PassRateDelta != regressed[j].Delta.PassRateDelta {
			return regressed[i].Delta.PassRateDelta < regressed[j].Delta.PassRateDelta
		}
		return regressed[i].ScenarioID < regressed[j].ScenarioID
	})
	return improved, regressed
}
