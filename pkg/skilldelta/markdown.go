package skilldelta

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RenderMarkdown renders a benchmark document as a stable Markdown report.
func RenderMarkdown(benchmark Benchmark) string {
	var b strings.Builder

	b.WriteString("# Skill Delta Benchmark\n\n")
	if benchmark.Metadata.GeneratedAt != "" {
		b.WriteString(fmt.Sprintf("Generated: %s\n\n", benchmark.Metadata.GeneratedAt))
	}

	b.WriteString("## Summary\n\n")
	b.WriteString("| Metric | Without skill | With skill | Delta |\n")
	b.WriteString("| --- | ---: | ---: | ---: |\n")
	writeSummaryRow(&b, "Pass rate (%)", benchmark.Summary.WithoutSkill.PassRatePct, benchmark.Summary.WithSkill.PassRatePct, benchmark.Summary.Delta.PassRatePct, 2)
	writeSummaryRow(&b, "Compliance (%)", benchmark.Summary.WithoutSkill.CompliancePct, benchmark.Summary.WithSkill.CompliancePct, benchmark.Summary.Delta.CompliancePct, 2)
	writeSummaryRow(&b, "Duration (s)", benchmark.Summary.WithoutSkill.DurationSeconds, benchmark.Summary.WithSkill.DurationSeconds, benchmark.Summary.Delta.DurationSeconds, 2)
	writeSummaryRow(&b, "Total tokens", benchmark.Summary.WithoutSkill.TotalTokens, benchmark.Summary.WithSkill.TotalTokens, benchmark.Summary.Delta.TotalTokens, 2)
	writeSummaryRow(&b, "Estimated cost (USD)", benchmark.Summary.WithoutSkill.EstimatedCostUSD, benchmark.Summary.WithSkill.EstimatedCostUSD, benchmark.Summary.Delta.EstimatedCostUSD, 4)
	writeSummaryRow(&b, "Evidra score", benchmark.Summary.WithoutSkill.Score, benchmark.Summary.WithSkill.Score, benchmark.Summary.Delta.Score, 2)

	b.WriteString("\n## Pairs\n\n")
	b.WriteString("| Scenario | Model | Repeat | Without | With | Compliance Δ | Token Δ | Cost Δ | Score Δ |\n")
	b.WriteString("| --- | --- | ---: | --- | --- | ---: | ---: | ---: | ---: |\n")
	for _, pair := range benchmark.Pairs {
		b.WriteString(fmt.Sprintf("| %s | %s | %d | %s | %s | %s | %s | %s | %s |\n",
			pair.ScenarioID,
			pair.Model,
			pair.Repeat,
			boolLabel(pair.WithoutSkill.Passed),
			boolLabel(pair.WithSkill.Passed),
			signed(pair.ComplianceDeltaPct, 2),
			signed(pair.TokenDelta.TotalTokens, 0),
			signed(pair.CostDeltaUSD, 4),
			signed(pair.ScoreDelta, 2),
		))
	}

	return b.String()
}

// WriteMarkdown writes benchmark.md to disk.
func WriteMarkdown(path string, benchmark Benchmark) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(RenderMarkdown(benchmark)), 0o644)
}

func writeSummaryRow(b *strings.Builder, label string, without, with NumericSummary, delta float64, precision int) {
	b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
		label,
		summaryCell(without, precision),
		summaryCell(with, precision),
		signed(delta, precision),
	))
}

func summaryCell(summary NumericSummary, precision int) string {
	return fmt.Sprintf("%.*f +/- %.*f", precision, summary.Mean, precision, summary.Stddev)
}

func boolLabel(passed bool) string {
	if passed {
		return "PASS"
	}
	return "FAIL"
}

func signed[T ~int | ~float64](value T, precision int) string {
	switch v := any(value).(type) {
	case int:
		return fmt.Sprintf("%+d", v)
	case float64:
		return fmt.Sprintf("%+.*f", precision, v)
	default:
		return ""
	}
}
