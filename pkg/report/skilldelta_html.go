package report

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"samebits.com/evidra-infra-bench/pkg/skilldelta"
)

//go:embed templates/skilldelta.html.tmpl
var skillDeltaHTMLFS embed.FS

// WriteSkillDeltaHTML writes a standalone HTML report for a skill-delta benchmark.
func WriteSkillDeltaHTML(outputPath string, benchmark skilldelta.Benchmark) error {
	tmpl, err := template.New("skilldelta").ParseFS(skillDeltaHTMLFS, "templates/skilldelta.html.tmpl")
	if err != nil {
		return fmt.Errorf("parse skill-delta template: %w", err)
	}

	view := buildSkillDeltaHTMLView(outputPath, benchmark)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "skilldelta.html.tmpl", view); err != nil {
		return fmt.Errorf("render skill-delta html: %w", err)
	}
	return os.WriteFile(outputPath, buf.Bytes(), 0o644)
}

type skillDeltaHTMLView struct {
	GeneratedAt string
	PairCount   int
	Summary     []skillDeltaSummaryRow
	Cards       []skillDeltaCard
	Pairs       []skillDeltaPairRow
}

type skillDeltaSummaryRow struct {
	Label   string
	Without string
	With    string
	Delta   string
}

type skillDeltaCard struct {
	Label string
	Value string
}

type skillDeltaPairRow struct {
	ScenarioID      string
	Model           string
	Repeat          int
	WithoutVerdict  string
	WithVerdict     string
	ComplianceDelta string
	TokenDelta      string
	CostDelta       string
	ScoreDelta      string
	WithoutBand     string
	WithBand        string
	Signals         []string
	WithoutRunLink  string
	WithRunLink     string
}

func buildSkillDeltaHTMLView(outputPath string, benchmark skilldelta.Benchmark) skillDeltaHTMLView {
	view := skillDeltaHTMLView{
		GeneratedAt: benchmark.Metadata.GeneratedAt,
		PairCount:   benchmark.Summary.PairCount,
		Summary: []skillDeltaSummaryRow{
			buildSummaryRow("Pass rate (%)", benchmark.Summary.WithoutSkill.PassRatePct, benchmark.Summary.WithSkill.PassRatePct, benchmark.Summary.Delta.PassRatePct, 2),
			buildSummaryRow("Compliance (%)", benchmark.Summary.WithoutSkill.CompliancePct, benchmark.Summary.WithSkill.CompliancePct, benchmark.Summary.Delta.CompliancePct, 2),
			buildSummaryRow("Duration (s)", benchmark.Summary.WithoutSkill.DurationSeconds, benchmark.Summary.WithSkill.DurationSeconds, benchmark.Summary.Delta.DurationSeconds, 2),
			buildSummaryRow("Total tokens", benchmark.Summary.WithoutSkill.TotalTokens, benchmark.Summary.WithSkill.TotalTokens, benchmark.Summary.Delta.TotalTokens, 2),
			buildSummaryRow("Estimated cost (USD)", benchmark.Summary.WithoutSkill.EstimatedCostUSD, benchmark.Summary.WithSkill.EstimatedCostUSD, benchmark.Summary.Delta.EstimatedCostUSD, 4),
			buildSummaryRow("Evidra score", benchmark.Summary.WithoutSkill.Score, benchmark.Summary.WithSkill.Score, benchmark.Summary.Delta.Score, 2),
		},
		Cards: []skillDeltaCard{
			{Label: "Pass rate delta", Value: signedFloat(benchmark.Summary.Delta.PassRatePct, 2)},
			{Label: "Compliance delta", Value: signedFloat(benchmark.Summary.Delta.CompliancePct, 2)},
			{Label: "Token delta", Value: signedFloat(benchmark.Summary.Delta.TotalTokens, 0)},
			{Label: "Cost delta", Value: signedFloat(benchmark.Summary.Delta.EstimatedCostUSD, 4)},
			{Label: "Duration delta", Value: signedFloat(benchmark.Summary.Delta.DurationSeconds, 2)},
			{Label: "Score delta", Value: signedFloat(benchmark.Summary.Delta.Score, 2)},
		},
	}

	for _, pair := range benchmark.Pairs {
		signals := append([]string(nil), pair.WithSkill.Scorecard.Signals...)
		if len(signals) == 0 {
			for name, count := range pair.WithSkill.Scorecard.SignalCounts {
				if count > 0 {
					signals = append(signals, name)
				}
			}
			sort.Strings(signals)
		}

		view.Pairs = append(view.Pairs, skillDeltaPairRow{
			ScenarioID:      pair.ScenarioID,
			Model:           pair.Model,
			Repeat:          pair.Repeat,
			WithoutVerdict:  verdictLabel(pair.WithoutSkill.Passed),
			WithVerdict:     verdictLabel(pair.WithSkill.Passed),
			ComplianceDelta: signedFloat(pair.ComplianceDeltaPct, 2),
			TokenDelta:      signedFloat(float64(pair.TokenDelta.TotalTokens), 0),
			CostDelta:       signedFloat(pair.CostDeltaUSD, 4),
			ScoreDelta:      signedFloat(pair.ScoreDelta, 2),
			WithoutBand:     pair.WithoutSkill.Scorecard.Band,
			WithBand:        pair.WithSkill.Scorecard.Band,
			Signals:         signals,
			WithoutRunLink:  relativeLink(outputPath, pair.Paths.WithoutSkillRunDir),
			WithRunLink:     relativeLink(outputPath, pair.Paths.WithSkillRunDir),
		})
	}

	return view
}

func buildSummaryRow(label string, without, with skilldelta.NumericSummary, delta float64, precision int) skillDeltaSummaryRow {
	return skillDeltaSummaryRow{
		Label:   label,
		Without: fmt.Sprintf("%.*f +/- %.*f", precision, without.Mean, precision, without.Stddev),
		With:    fmt.Sprintf("%.*f +/- %.*f", precision, with.Mean, precision, with.Stddev),
		Delta:   signedFloat(delta, precision),
	}
}

func signedFloat(value float64, precision int) string {
	return fmt.Sprintf("%+.*f", precision, value)
}

func verdictLabel(passed bool) string {
	if passed {
		return "PASS"
	}
	return "FAIL"
}

func relativeLink(outputPath, target string) string {
	if strings.TrimSpace(target) == "" {
		return ""
	}
	rel, err := filepath.Rel(filepath.Dir(outputPath), target)
	if err != nil {
		return target
	}
	return filepath.ToSlash(rel)
}
