package skilldelta

import "math"

// Aggregate computes benchmark summary statistics across all pair results.
func Aggregate(pairs []PairResult) BenchmarkSummary {
	summary := BenchmarkSummary{
		PairCount: len(pairs),
	}
	if len(pairs) == 0 {
		return summary
	}

	withoutPassRates := make([]float64, 0, len(pairs))
	withPassRates := make([]float64, 0, len(pairs))
	withoutCompliance := make([]float64, 0, len(pairs))
	withCompliance := make([]float64, 0, len(pairs))
	withoutDuration := make([]float64, 0, len(pairs))
	withDuration := make([]float64, 0, len(pairs))
	withoutTokens := make([]float64, 0, len(pairs))
	withTokens := make([]float64, 0, len(pairs))
	withoutCost := make([]float64, 0, len(pairs))
	withCost := make([]float64, 0, len(pairs))
	withoutScore := make([]float64, 0, len(pairs))
	withScore := make([]float64, 0, len(pairs))

	for _, pair := range pairs {
		withoutPassRates = append(withoutPassRates, passRate(pair.WithoutSkill.Passed))
		withPassRates = append(withPassRates, passRate(pair.WithSkill.Passed))
		withoutCompliance = append(withoutCompliance, pair.WithoutSkill.Protocol.ComplianceRatePct)
		withCompliance = append(withCompliance, pair.WithSkill.Protocol.ComplianceRatePct)
		withoutDuration = append(withoutDuration, pair.WithoutSkill.DurationSeconds)
		withDuration = append(withDuration, pair.WithSkill.DurationSeconds)
		withoutTokens = append(withoutTokens, float64(pair.WithoutSkill.TotalTokens))
		withTokens = append(withTokens, float64(pair.WithSkill.TotalTokens))
		withoutCost = append(withoutCost, pair.WithoutSkill.EstimatedCostUSD)
		withCost = append(withCost, pair.WithSkill.EstimatedCostUSD)
		if pair.WithoutSkill.Scorecard.Score != nil {
			withoutScore = append(withoutScore, *pair.WithoutSkill.Scorecard.Score)
		}
		if pair.WithSkill.Scorecard.Score != nil {
			withScore = append(withScore, *pair.WithSkill.Scorecard.Score)
		}
	}

	summary.WithoutSkill = ConfigurationSummary{
		PassRatePct:      summarize(withoutPassRates),
		CompliancePct:    summarize(withoutCompliance),
		DurationSeconds:  summarize(withoutDuration),
		TotalTokens:      summarize(withoutTokens),
		EstimatedCostUSD: summarize(withoutCost),
		Score:            summarize(withoutScore),
	}
	summary.WithSkill = ConfigurationSummary{
		PassRatePct:      summarize(withPassRates),
		CompliancePct:    summarize(withCompliance),
		DurationSeconds:  summarize(withDuration),
		TotalTokens:      summarize(withTokens),
		EstimatedCostUSD: summarize(withCost),
		Score:            summarize(withScore),
	}
	summary.Delta = DeltaSummary{
		PassRatePct:      round4(summary.WithSkill.PassRatePct.Mean - summary.WithoutSkill.PassRatePct.Mean),
		CompliancePct:    round4(summary.WithSkill.CompliancePct.Mean - summary.WithoutSkill.CompliancePct.Mean),
		DurationSeconds:  round4(summary.WithSkill.DurationSeconds.Mean - summary.WithoutSkill.DurationSeconds.Mean),
		TotalTokens:      round4(summary.WithSkill.TotalTokens.Mean - summary.WithoutSkill.TotalTokens.Mean),
		EstimatedCostUSD: round4(summary.WithSkill.EstimatedCostUSD.Mean - summary.WithoutSkill.EstimatedCostUSD.Mean),
		Score:            round4(summary.WithSkill.Score.Mean - summary.WithoutSkill.Score.Mean),
	}

	return summary
}

func summarize(values []float64) NumericSummary {
	if len(values) == 0 {
		return NumericSummary{}
	}

	mean := 0.0
	min := values[0]
	max := values[0]
	for _, value := range values {
		mean += value
		if value < min {
			min = value
		}
		if value > max {
			max = value
		}
	}
	mean /= float64(len(values))

	stddev := 0.0
	if len(values) > 1 {
		var variance float64
		for _, value := range values {
			delta := value - mean
			variance += delta * delta
		}
		variance /= float64(len(values) - 1)
		stddev = math.Sqrt(variance)
	}

	return NumericSummary{
		Mean:   round4(mean),
		Stddev: round4(stddev),
		Min:    round4(min),
		Max:    round4(max),
	}
}

func passRate(passed bool) float64 {
	if passed {
		return 100
	}
	return 0
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}
