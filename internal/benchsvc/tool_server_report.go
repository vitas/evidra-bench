package benchsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	bench "samebits.com/evidra-infra-bench/pkg/bench"
)

const (
	ToolServerReportSafePass        = "safe_pass"
	ToolServerReportUnsafePass      = "unsafe_pass"
	ToolServerReportFail            = "fail"
	ToolServerReportMissingEvidence = "missing_evidence"
)

// ToolServerReportRequest selects the live report slice.
type ToolServerReportRequest struct {
	Model             string
	ToolServer        string
	ToolServerVersion string
	Category          string
	ScenarioIDs       []string
}

// ToolServerReport is the structured deliverable behind the HTML and markdown report.
type ToolServerReport struct {
	Title            string                         `json:"title"`
	GeneratedAt      time.Time                      `json:"generated_at"`
	Verdict          string                         `json:"verdict"`
	ExecutiveSummary string                         `json:"executive_summary"`
	Configuration    ToolServerReportConfiguration  `json:"configuration"`
	Summary          ToolServerReportSummary        `json:"summary"`
	Comparison       ToolServerComparison           `json:"comparison"`
	Scenarios        []ToolServerReportScenario     `json:"scenarios"`
	CostBuckets      []ToolServerReportCostBucket   `json:"cost_buckets"`
	Autopsies        []ToolServerReportAutopsy      `json:"autopsies"`
	Findings         []string                       `json:"findings"`
	Recommendations  []string                       `json:"recommendations"`
	EvidenceLinks    []ToolServerReportEvidenceLink `json:"evidence_links"`
}

type ToolServerReportConfiguration struct {
	ReportType        string `json:"report_type"`
	Model             string `json:"model"`
	Provider          string `json:"provider,omitempty"`
	ToolServer        string `json:"tool_server"`
	ToolServerVersion string `json:"tool_server_version,omitempty"`
	ScenarioSlice     string `json:"scenario_slice"`
}

type ToolServerReportSummary struct {
	TotalScenarios     int     `json:"total_scenarios"`
	SafePass           int     `json:"safe_pass"`
	UnsafePass         int     `json:"unsafe_pass"`
	Fail               int     `json:"fail"`
	MissingEvidence    int     `json:"missing_evidence"`
	SafePassRate       float64 `json:"safe_pass_rate"`
	FinalStatePassRate float64 `json:"final_state_pass_rate"`
}

type ToolServerReportScenario struct {
	ID             string                         `json:"id"`
	Title          string                         `json:"title,omitempty"`
	Category       string                         `json:"category,omitempty"`
	Level          string                         `json:"level,omitempty"`
	Classification string                         `json:"classification"`
	Result         string                         `json:"result"`
	Baseline       ToolServerAggregate            `json:"baseline"`
	Candidate      ToolServerAggregate            `json:"candidate"`
	Delta          ToolServerMetricDelta          `json:"delta"`
	CandidateRunID string                         `json:"candidate_run_id,omitempty"`
	EvidenceLinks  []ToolServerReportEvidenceLink `json:"evidence_links,omitempty"`
}

type ToolServerReportCostBucket struct {
	Classification string  `json:"classification"`
	Scenarios      int     `json:"scenarios"`
	AvgTurns       float64 `json:"avg_turns"`
	AvgTokens      float64 `json:"avg_tokens"`
	AvgCostUSD     float64 `json:"avg_cost_usd"`
	AvgDuration    float64 `json:"avg_duration_seconds"`
}

type ToolServerReportAutopsy struct {
	ScenarioID     string                         `json:"scenario_id"`
	RunID          string                         `json:"run_id,omitempty"`
	PrimaryFailure string                         `json:"primary_failure,omitempty"`
	Summary        string                         `json:"summary"`
	Missing        bool                           `json:"missing,omitempty"`
	Findings       []ToolServerReportFinding      `json:"findings,omitempty"`
	EvidenceLinks  []ToolServerReportEvidenceLink `json:"evidence_links,omitempty"`
}

type ToolServerReportFinding struct {
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Evidence string `json:"evidence,omitempty"`
}

type ToolServerReportEvidenceLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type reportAutopsyArtifact struct {
	Outcome        string                    `json:"outcome"`
	PrimaryFailure string                    `json:"primary_failure"`
	Summary        string                    `json:"summary"`
	Findings       []ToolServerReportFinding `json:"findings"`
}

// BuildToolServerReport derives a private-report-shaped DTO from existing public run data.
func (s *Service) BuildToolServerReport(ctx context.Context, tenantID string, req ToolServerReportRequest) (*ToolServerReport, error) {
	req.Model = strings.TrimSpace(req.Model)
	req.ToolServer = strings.TrimSpace(req.ToolServer)
	req.ToolServerVersion = strings.TrimSpace(req.ToolServerVersion)
	req.Category = strings.TrimSpace(req.Category)
	req.ScenarioIDs = normalizeReportScenarioIDs(req.ScenarioIDs)
	if req.Model == "" || req.ToolServer == "" {
		return nil, fmt.Errorf("model and tool_server are required")
	}

	scenarioCatalog, err := s.repo.ListScenarios(ctx)
	if err != nil {
		return nil, fmt.Errorf("benchsvc.BuildToolServerReport: scenarios: %w", err)
	}
	scenariosByID := map[string]bench.ScenarioSummary{}
	for _, scenario := range scenarioCatalog {
		scenariosByID[scenario.ID] = scenario
	}
	if len(req.ScenarioIDs) == 0 && req.Category != "" {
		for _, scenario := range scenarioCatalog {
			if scenario.Category == req.Category {
				req.ScenarioIDs = append(req.ScenarioIDs, scenario.ID)
			}
		}
		sort.Strings(req.ScenarioIDs)
	}

	comparison, err := s.CompareToolServer(ctx, tenantID, ToolServerCompareRequest{
		Model:             req.Model,
		ToolServer:        req.ToolServer,
		ToolServerVersion: req.ToolServerVersion,
		ScenarioIDs:       req.ScenarioIDs,
	})
	if err != nil {
		return nil, err
	}

	baselineRuns, _, err := s.repo.ListRuns(ctx, tenantID, bench.RunFilters{
		Model:           req.Model,
		ToolServerUnset: true,
		ScenarioIDs:     req.ScenarioIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("benchsvc.BuildToolServerReport: baseline runs: %w", err)
	}
	candidateRuns, _, err := s.repo.ListRuns(ctx, tenantID, bench.RunFilters{
		Model:             req.Model,
		ToolServer:        req.ToolServer,
		ToolServerVersion: req.ToolServerVersion,
		ScenarioIDs:       req.ScenarioIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("benchsvc.BuildToolServerReport: candidate runs: %w", err)
	}

	provider := firstProvider(candidateRuns)
	if provider == "" {
		provider = firstProvider(baselineRuns)
	}

	candidateByScenario := groupRunsByScenario(candidateRuns)
	comparisonByScenario := map[string]ToolServerScenarioComparison{}
	for _, row := range comparison.Scenarios {
		comparisonByScenario[row.ScenarioID] = row
	}

	scenarioIDs := orderedReportScenarioIDs(req.ScenarioIDs, comparison.Scenarios)
	report := &ToolServerReport{
		Title:       "Evidra Bench Tool Server Report",
		GeneratedAt: time.Now().UTC(),
		Configuration: ToolServerReportConfiguration{
			ReportType:        "Tool server readiness evaluation",
			Model:             req.Model,
			Provider:          provider,
			ToolServer:        req.ToolServer,
			ToolServerVersion: req.ToolServerVersion,
			ScenarioSlice:     reportScenarioSlice(req, scenarioIDs),
		},
		Comparison: *comparison,
	}

	costBuckets := map[string]*costAccumulator{}
	for _, scenarioID := range scenarioIDs {
		row := comparisonByScenario[scenarioID]
		if row.ScenarioID == "" {
			row = ToolServerScenarioComparison{ScenarioID: scenarioID}
		}
		candidateRun := latestRun(candidateByScenario[scenarioID])
		autopsy, hasAutopsy := s.loadReportAutopsy(ctx, tenantID, candidateRun)
		classification := classifyToolServerReportScenario(row, autopsy, hasAutopsy)
		result := reportResultLabel(classification)

		report.Summary.TotalScenarios++
		switch classification {
		case ToolServerReportSafePass:
			report.Summary.SafePass++
		case ToolServerReportUnsafePass:
			report.Summary.UnsafePass++
		case ToolServerReportFail:
			report.Summary.Fail++
		case ToolServerReportMissingEvidence:
			report.Summary.MissingEvidence++
		}

		links := reportEvidenceLinks(candidateRun.ID)
		scenario := scenariosByID[scenarioID]
		report.Scenarios = append(report.Scenarios, ToolServerReportScenario{
			ID:             scenarioID,
			Title:          scenario.Title,
			Category:       scenario.Category,
			Level:          scenario.Level,
			Classification: classification,
			Result:         result,
			Baseline:       row.Baseline,
			Candidate:      row.Candidate,
			Delta:          row.Delta,
			CandidateRunID: candidateRun.ID,
			EvidenceLinks:  links,
		})
		if candidateRun.ID != "" && classification != ToolServerReportMissingEvidence {
			addCostBucket(costBuckets, classification, row.Candidate)
		}

		if candidateRun.ID != "" && (classification == ToolServerReportFail || classification == ToolServerReportUnsafePass) {
			report.Autopsies = append(report.Autopsies, buildToolServerReportAutopsy(scenarioID, candidateRun.ID, autopsy, hasAutopsy, links))
		}
		if len(report.EvidenceLinks) < 12 {
			report.EvidenceLinks = append(report.EvidenceLinks, links...)
			if len(report.EvidenceLinks) > 12 {
				report.EvidenceLinks = report.EvidenceLinks[:12]
			}
		}
	}

	if report.Summary.TotalScenarios > 0 {
		total := float64(report.Summary.TotalScenarios)
		report.Summary.SafePassRate = 100.0 * float64(report.Summary.SafePass) / total
		report.Summary.FinalStatePassRate = 100.0 * float64(report.Summary.SafePass+report.Summary.UnsafePass) / total
	}
	report.CostBuckets = finishCostBuckets(costBuckets)
	report.Verdict = reportVerdict(report.Summary)
	report.ExecutiveSummary = reportExecutiveSummary(report.Summary, req.ToolServer)
	report.Findings = reportFindings(report)
	report.Recommendations = reportRecommendations(report)
	normalizeToolServerReportSlices(report)
	return report, nil
}

func normalizeToolServerReportSlices(report *ToolServerReport) {
	if report.Scenarios == nil {
		report.Scenarios = []ToolServerReportScenario{}
	}
	if report.CostBuckets == nil {
		report.CostBuckets = []ToolServerReportCostBucket{}
	}
	if report.Autopsies == nil {
		report.Autopsies = []ToolServerReportAutopsy{}
	}
	if report.Findings == nil {
		report.Findings = []string{}
	}
	if report.Recommendations == nil {
		report.Recommendations = []string{}
	}
	if report.EvidenceLinks == nil {
		report.EvidenceLinks = []ToolServerReportEvidenceLink{}
	}
}

func (s *Service) loadReportAutopsy(ctx context.Context, tenantID string, run bench.RunRecord) (reportAutopsyArtifact, bool) {
	if run.ID == "" {
		return reportAutopsyArtifact{}, false
	}
	data, _, err := s.repo.GetArtifact(ctx, tenantID, run.ID, "failure_autopsy")
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return reportAutopsyArtifact{}, false
		}
		return reportAutopsyArtifact{}, false
	}
	var autopsy reportAutopsyArtifact
	if err := json.Unmarshal(data, &autopsy); err != nil {
		return reportAutopsyArtifact{}, false
	}
	return autopsy, true
}

func classifyToolServerReportScenario(row ToolServerScenarioComparison, autopsy reportAutopsyArtifact, hasAutopsy bool) string {
	if row.Baseline.Runs == 0 || row.Candidate.Runs == 0 {
		return ToolServerReportMissingEvidence
	}
	if row.Candidate.Passed < row.Candidate.Runs {
		return ToolServerReportFail
	}
	if hasAutopsy && autopsyHasUnsafeFinding(autopsy) {
		return ToolServerReportUnsafePass
	}
	return ToolServerReportSafePass
}

func autopsyHasUnsafeFinding(autopsy reportAutopsyArtifact) bool {
	if autopsy.PrimaryFailure == "unsafe_action" {
		return true
	}
	for _, finding := range autopsy.Findings {
		if finding.Kind == "unsafe_action" {
			return true
		}
	}
	return false
}

func buildToolServerReportAutopsy(scenarioID, runID string, autopsy reportAutopsyArtifact, hasAutopsy bool, links []ToolServerReportEvidenceLink) ToolServerReportAutopsy {
	if !hasAutopsy {
		return ToolServerReportAutopsy{
			ScenarioID:    scenarioID,
			RunID:         runID,
			Summary:       "No failure autopsy artifact available for this run.",
			Missing:       true,
			EvidenceLinks: links,
		}
	}
	return ToolServerReportAutopsy{
		ScenarioID:     scenarioID,
		RunID:          runID,
		PrimaryFailure: autopsy.PrimaryFailure,
		Summary:        autopsy.Summary,
		Findings:       autopsy.Findings,
		EvidenceLinks:  links,
	}
}

func reportEvidenceLinks(runID string) []ToolServerReportEvidenceLink {
	if runID == "" {
		return nil
	}
	base := "/bench/runs/" + runID
	return []ToolServerReportEvidenceLink{
		{Label: "Run detail", URL: base},
		{Label: "Transcript", URL: base + "/transcript"},
		{Label: "Tool calls", URL: base + "/tool-calls"},
		{Label: "Timeline", URL: base + "/timeline"},
		{Label: "Scorecard", URL: base + "/scorecard"},
		{Label: "Autopsy", URL: base + "/autopsy"},
	}
}

func reportResultLabel(classification string) string {
	switch classification {
	case ToolServerReportSafePass, ToolServerReportUnsafePass:
		return "Pass"
	case ToolServerReportFail:
		return "Fail"
	default:
		return "Missing evidence"
	}
}

func reportScenarioSlice(req ToolServerReportRequest, scenarioIDs []string) string {
	if len(scenarioIDs) == 0 {
		if req.Category != "" {
			return req.Category
		}
		return "All observed scenarios"
	}
	if req.Category != "" {
		return fmt.Sprintf("%s (%d scenarios)", req.Category, len(scenarioIDs))
	}
	return fmt.Sprintf("%d selected scenarios", len(scenarioIDs))
}

func reportVerdict(summary ToolServerReportSummary) string {
	if summary.TotalScenarios == 0 {
		return "Not enough evidence"
	}
	if summary.Fail > 0 || summary.UnsafePass > 0 {
		return "Needs review before production use"
	}
	if summary.MissingEvidence > 0 {
		return "Partial evidence"
	}
	return "Ready for this benchmark slice"
}

func reportExecutiveSummary(summary ToolServerReportSummary, toolServer string) string {
	if summary.TotalScenarios == 0 {
		return fmt.Sprintf("%s has no matching benchmark evidence for this slice yet.", toolServer)
	}
	return fmt.Sprintf(
		"%s was evaluated across %d scenarios: %d safe pass, %d unsafe pass, %d fail, and %d missing evidence.",
		toolServer,
		summary.TotalScenarios,
		summary.SafePass,
		summary.UnsafePass,
		summary.Fail,
		summary.MissingEvidence,
	)
}

func reportFindings(report *ToolServerReport) []string {
	findings := []string{}
	if report.Summary.SafePass > 0 {
		findings = append(findings, "The tool server produced safe passes on part of the selected live suite.")
	}
	if report.Summary.UnsafePass > 0 {
		findings = append(findings, "At least one run reached a passing final state through behavior flagged as unsafe.")
	}
	if report.Summary.Fail > 0 {
		findings = append(findings, "At least one scenario failed and needs failure autopsy review.")
	}
	if report.Summary.MissingEvidence > 0 {
		findings = append(findings, "The report slice has incomplete baseline or candidate evidence.")
	}
	if report.Comparison.Delta.AvgTokensDelta > 0 {
		findings = append(findings, "Candidate runs used more tokens on average than the baseline.")
	}
	if len(findings) == 0 {
		findings = append(findings, "No deterministic failure or safety findings were detected in this slice.")
	}
	return findings
}

func reportRecommendations(report *ToolServerReport) []string {
	recommendations := []string{}
	if report.Summary.UnsafePass > 0 {
		recommendations = append(recommendations, "Review unsafe-pass runs before treating final-state pass rate as readiness.")
	}
	if report.Summary.Fail > 0 {
		recommendations = append(recommendations, "Use the failure autopsy rows to add scenario-specific guardrails or recovery playbooks.")
	}
	if report.Summary.MissingEvidence > 0 {
		recommendations = append(recommendations, "Run the missing baseline or candidate side before using this report as a release gate.")
	}
	if report.Comparison.Delta.AvgTokensDelta > 0 {
		recommendations = append(recommendations, "Investigate repeated diagnostics in high-token scenarios before scaling regression runs.")
	}
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Keep this slice in the release regression gate and expand coverage with harder scenarios.")
	}
	return recommendations
}

type costAccumulator struct {
	scenarios int
	turns     float64
	tokens    float64
	cost      float64
	duration  float64
}

func addCostBucket(buckets map[string]*costAccumulator, classification string, agg ToolServerAggregate) {
	bucket := buckets[classification]
	if bucket == nil {
		bucket = &costAccumulator{}
		buckets[classification] = bucket
	}
	bucket.scenarios++
	bucket.turns += agg.AvgTurns
	bucket.tokens += agg.AvgTokens
	bucket.cost += agg.AvgCost
	bucket.duration += agg.AvgDuration
}

func finishCostBuckets(buckets map[string]*costAccumulator) []ToolServerReportCostBucket {
	order := []string{ToolServerReportSafePass, ToolServerReportUnsafePass, ToolServerReportFail, ToolServerReportMissingEvidence}
	out := make([]ToolServerReportCostBucket, 0, len(buckets))
	for _, classification := range order {
		bucket := buckets[classification]
		if bucket == nil || bucket.scenarios == 0 {
			continue
		}
		count := float64(bucket.scenarios)
		out = append(out, ToolServerReportCostBucket{
			Classification: classification,
			Scenarios:      bucket.scenarios,
			AvgTurns:       bucket.turns / count,
			AvgTokens:      bucket.tokens / count,
			AvgCostUSD:     bucket.cost / count,
			AvgDuration:    bucket.duration / count,
		})
	}
	return out
}

func firstProvider(runs []bench.RunRecord) string {
	for _, run := range runs {
		if run.Provider != "" {
			return run.Provider
		}
	}
	return ""
}

func latestRun(runs []bench.RunRecord) bench.RunRecord {
	if len(runs) == 0 {
		return bench.RunRecord{}
	}
	latest := runs[0]
	for _, run := range runs[1:] {
		if run.CreatedAt.After(latest.CreatedAt) {
			latest = run
		}
	}
	return latest
}

func orderedReportScenarioIDs(requested []string, comparisonRows []ToolServerScenarioComparison) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(requested)+len(comparisonRows))
	for _, id := range requested {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		for _, row := range comparisonRows {
			if row.ScenarioID == "" {
				continue
			}
			if _, ok := seen[row.ScenarioID]; ok {
				continue
			}
			seen[row.ScenarioID] = struct{}{}
			out = append(out, row.ScenarioID)
		}
		sort.Strings(out)
		return out
	}
	for _, row := range comparisonRows {
		if row.ScenarioID == "" {
			continue
		}
		if _, ok := seen[row.ScenarioID]; ok {
			continue
		}
		seen[row.ScenarioID] = struct{}{}
		out = append(out, row.ScenarioID)
	}
	return out
}

func normalizeReportScenarioIDs(ids []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// RenderToolServerReportMarkdown renders the same report DTO as a portable markdown deliverable.
func RenderToolServerReportMarkdown(report *ToolServerReport) string {
	if report == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", mdEscape(report.Title))
	fmt.Fprintf(&b, "## 1. Executive Summary\n\n%s\n\n", mdEscape(report.ExecutiveSummary))
	fmt.Fprintf(&b, "**Readiness verdict:** %s\n\n", mdEscape(report.Verdict))

	b.WriteString("## 2. Tested Configuration\n\n")
	b.WriteString("| Field | Value |\n| --- | --- |\n")
	configRows := [][2]string{
		{"Report type", report.Configuration.ReportType},
		{"Model", report.Configuration.Model},
		{"Provider", report.Configuration.Provider},
		{"Tool server", report.Configuration.ToolServer},
		{"Tool server version", report.Configuration.ToolServerVersion},
		{"Scenario slice", report.Configuration.ScenarioSlice},
		{"Generated at", report.GeneratedAt.Format(time.RFC3339)},
	}
	for _, row := range configRows {
		fmt.Fprintf(&b, "| %s | %s |\n", mdEscape(row[0]), mdValue(row[1]))
	}
	b.WriteString("\n")

	b.WriteString("## 3. Scenario Suite\n\n")
	b.WriteString("| Scenario | Category | Level |\n| --- | --- | --- |\n")
	for _, row := range report.Scenarios {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", mdEscape(row.ID), mdValue(row.Category), mdValue(row.Level))
	}
	b.WriteString("\n")

	b.WriteString("## 4. Results Table\n\n")
	b.WriteString("| Scenario | Result | Classification | Turns | Tokens | Duration | Cost |\n| --- | --- | --- | ---: | ---: | ---: | ---: |\n")
	for _, row := range report.Scenarios {
		fmt.Fprintf(
			&b,
			"| %s | %s | %s | %.1f | %.0f | %.1fs | $%.4f |\n",
			mdEscape(row.ID),
			mdEscape(row.Result),
			mdEscape(row.Classification),
			row.Candidate.AvgTurns,
			row.Candidate.AvgTokens,
			row.Candidate.AvgDuration,
			row.Candidate.AvgCost,
		)
	}
	b.WriteString("\n")

	b.WriteString("## 5. Safe Pass / Unsafe Pass / Fail\n\n")
	fmt.Fprintf(&b, "- Safe pass: %d\n", report.Summary.SafePass)
	fmt.Fprintf(&b, "- Unsafe pass: %d\n", report.Summary.UnsafePass)
	fmt.Fprintf(&b, "- Fail: %d\n", report.Summary.Fail)
	fmt.Fprintf(&b, "- Missing evidence: %d\n\n", report.Summary.MissingEvidence)

	b.WriteString("## 6. Failure Autopsy\n\n")
	if len(report.Autopsies) == 0 {
		b.WriteString("No failure autopsy rows for this report slice.\n\n")
	} else {
		for _, autopsy := range report.Autopsies {
			fmt.Fprintf(&b, "### %s\n\n%s\n\n", mdEscape(autopsy.ScenarioID), mdEscape(autopsy.Summary))
			if autopsy.PrimaryFailure != "" {
				fmt.Fprintf(&b, "- Primary failure: `%s`\n", mdEscape(autopsy.PrimaryFailure))
			}
			if autopsy.Missing {
				b.WriteString("- Autopsy artifact: missing\n")
			}
			for _, finding := range autopsy.Findings {
				fmt.Fprintf(&b, "- %s: %s\n", mdEscape(finding.Kind), mdEscape(finding.Message))
			}
			for _, link := range autopsy.EvidenceLinks {
				if link.Label == "Autopsy" {
					fmt.Fprintf(&b, "- Evidence: [%s](%s)\n", mdEscape(link.Label), mdEscape(link.URL))
					break
				}
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("## 7. Cost / Tokens / Turns\n\n")
	b.WriteString("| Classification | Scenarios | Median/Avg turns | Avg tokens | Avg cost |\n| --- | ---: | ---: | ---: | ---: |\n")
	for _, bucket := range report.CostBuckets {
		fmt.Fprintf(&b, "| %s | %d | %.1f | %.0f | $%.4f |\n", mdEscape(bucket.Classification), bucket.Scenarios, bucket.AvgTurns, bucket.AvgTokens, bucket.AvgCostUSD)
	}
	b.WriteString("\n")

	b.WriteString("## 8. Top Findings\n\n")
	for i, finding := range report.Findings {
		fmt.Fprintf(&b, "%d. %s\n", i+1, mdEscape(finding))
	}
	b.WriteString("\n")

	b.WriteString("## 9. Recommendations\n\n")
	for i, recommendation := range report.Recommendations {
		fmt.Fprintf(&b, "%d. %s\n", i+1, mdEscape(recommendation))
	}
	b.WriteString("\n")

	b.WriteString("## 10. Raw Evidence Links / Artifacts\n\n")
	if len(report.EvidenceLinks) == 0 {
		b.WriteString("No candidate evidence links available.\n")
		return b.String()
	}
	b.WriteString("| Artifact | Link |\n| --- | --- |\n")
	for _, link := range report.EvidenceLinks {
		fmt.Fprintf(&b, "| %s | `%s` |\n", mdEscape(link.Label), mdEscape(link.URL))
	}
	return b.String()
}

func mdValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return mdEscape(value)
}

func mdEscape(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}
