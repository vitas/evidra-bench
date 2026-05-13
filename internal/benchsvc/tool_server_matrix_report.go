package benchsvc

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ToolServerMatrixReportRequest selects a baseline plus multiple MCP tool-server arms.
type ToolServerMatrixReportRequest struct {
	Model              string
	ReportID           string
	ToolServers        []string
	ToolServerVersions []string
	ScenarioIDs        []string
}

// ToolServerMatrixReport is the public multi-arm report DTO.
type ToolServerMatrixReport struct {
	Title           string                         `json:"title"`
	GeneratedAt     time.Time                      `json:"generated_at"`
	Model           string                         `json:"model"`
	ReportID        string                         `json:"report_id"`
	ScenarioIDs     []string                       `json:"scenario_ids,omitempty"`
	Arms            []ToolServerMatrixArm          `json:"arms"`
	Summary         ToolServerMatrixSummary        `json:"summary"`
	Methodology     []string                       `json:"methodology"`
	Scenarios       []ToolServerMatrixScenario     `json:"scenarios"`
	Autopsies       []ToolServerMatrixAutopsy      `json:"autopsies"`
	Findings        []string                       `json:"findings"`
	Recommendations []string                       `json:"recommendations"`
	EvidenceLinks   []ToolServerReportEvidenceLink `json:"evidence_links"`
}

type ToolServerMatrixArm struct {
	ID                string              `json:"id"`
	Label             string              `json:"label"`
	Kind              string              `json:"kind"`
	ToolServer        string              `json:"tool_server,omitempty"`
	ToolServerVersion string              `json:"tool_server_version,omitempty"`
	Aggregate         ToolServerAggregate `json:"aggregate"`
}

type ToolServerMatrixSummary struct {
	TotalScenarios  int `json:"total_scenarios"`
	CandidateCells  int `json:"candidate_cells"`
	SafePass        int `json:"safe_pass"`
	UnsafePass      int `json:"unsafe_pass"`
	Fail            int `json:"fail"`
	MissingEvidence int `json:"missing_evidence"`
}

type ToolServerMatrixScenario struct {
	ID       string                        `json:"id"`
	Title    string                        `json:"title,omitempty"`
	Category string                        `json:"category,omitempty"`
	Level    string                        `json:"level,omitempty"`
	Arms     []ToolServerMatrixScenarioArm `json:"arms"`
}

type ToolServerMatrixScenarioArm struct {
	ArmID          string                         `json:"arm_id"`
	Label          string                         `json:"label"`
	Classification string                         `json:"classification"`
	Result         string                         `json:"result"`
	RunID          string                         `json:"run_id,omitempty"`
	Aggregate      ToolServerAggregate            `json:"aggregate"`
	EvidenceLinks  []ToolServerReportEvidenceLink `json:"evidence_links,omitempty"`
}

type ToolServerMatrixAutopsy struct {
	ArmID             string                         `json:"arm_id"`
	ToolServer        string                         `json:"tool_server"`
	ToolServerVersion string                         `json:"tool_server_version,omitempty"`
	ScenarioID        string                         `json:"scenario_id"`
	RunID             string                         `json:"run_id,omitempty"`
	PrimaryFailure    string                         `json:"primary_failure,omitempty"`
	Summary           string                         `json:"summary"`
	Missing           bool                           `json:"missing,omitempty"`
	Findings          []ToolServerReportFinding      `json:"findings,omitempty"`
	EvidenceLinks     []ToolServerReportEvidenceLink `json:"evidence_links,omitempty"`
}

// BuildToolServerMatrixReport composes multiple candidate-vs-baseline reports into one public matrix.
func (s *Service) BuildToolServerMatrixReport(ctx context.Context, tenantID string, req ToolServerMatrixReportRequest) (*ToolServerMatrixReport, error) {
	req.Model = strings.TrimSpace(req.Model)
	req.ReportID = strings.TrimSpace(req.ReportID)
	req.ToolServers = normalizeMatrixStrings(req.ToolServers)
	req.ToolServerVersions = normalizeMatrixStrings(req.ToolServerVersions)
	req.ScenarioIDs = normalizeReportScenarioIDs(req.ScenarioIDs)

	if req.Model == "" {
		return nil, fmt.Errorf("model is required")
	}
	if req.ReportID == "" {
		return nil, fmt.Errorf("report_id is required")
	}
	if len(req.ToolServers) == 0 {
		return nil, fmt.Errorf("tool_servers is required")
	}
	if len(req.ToolServerVersions) > 0 && len(req.ToolServerVersions) != len(req.ToolServers) {
		return nil, fmt.Errorf("tool_server_versions must match tool_servers length")
	}
	for len(req.ToolServerVersions) < len(req.ToolServers) {
		req.ToolServerVersions = append(req.ToolServerVersions, "")
	}

	candidateReports := make([]*ToolServerReport, 0, len(req.ToolServers))
	for i, toolServer := range req.ToolServers {
		candidateReport, err := s.BuildToolServerReport(ctx, tenantID, ToolServerReportRequest{
			Model:             req.Model,
			ReportID:          req.ReportID,
			ToolServer:        toolServer,
			ToolServerVersion: req.ToolServerVersions[i],
			ScenarioIDs:       req.ScenarioIDs,
		})
		if err != nil {
			return nil, fmt.Errorf("benchsvc.BuildToolServerMatrixReport: %s: %w", toolServer, err)
		}
		candidateReports = append(candidateReports, candidateReport)
	}

	scenarioIDs := orderedMatrixScenarioIDs(req.ScenarioIDs, candidateReports)
	report := &ToolServerMatrixReport{
		Title:       "Kubernetes MCP Server Readiness Report",
		GeneratedAt: time.Now().UTC(),
		Model:       req.Model,
		ReportID:    req.ReportID,
		ScenarioIDs: append([]string(nil), scenarioIDs...),
		Methodology: []string{
			"Every arm uses the same model, provider configuration, scenario slice, and report_id.",
			"The baseline arm uses direct Bench tools with an empty tool_server identity.",
			"Candidate arms use the selected MCP tool-server identity and version.",
			"Safe pass, unsafe pass, fail, and missing evidence classifications reuse the single-tool-server report rules.",
		},
	}

	first := candidateReports[0]
	report.Arms = append(report.Arms, ToolServerMatrixArm{
		ID:        "baseline",
		Label:     "Baseline",
		Kind:      "baseline",
		Aggregate: first.Comparison.Baseline,
	})
	for i, candidateReport := range candidateReports {
		report.Arms = append(report.Arms, ToolServerMatrixArm{
			ID:                req.ToolServers[i],
			Label:             req.ToolServers[i],
			Kind:              "candidate",
			ToolServer:        req.ToolServers[i],
			ToolServerVersion: req.ToolServerVersions[i],
			Aggregate:         candidateReport.Comparison.Candidate,
		})
		for _, autopsy := range candidateReport.Autopsies {
			report.Autopsies = append(report.Autopsies, ToolServerMatrixAutopsy{
				ArmID:             req.ToolServers[i],
				ToolServer:        req.ToolServers[i],
				ToolServerVersion: req.ToolServerVersions[i],
				ScenarioID:        autopsy.ScenarioID,
				RunID:             autopsy.RunID,
				PrimaryFailure:    autopsy.PrimaryFailure,
				Summary:           autopsy.Summary,
				Missing:           autopsy.Missing,
				Findings:          autopsy.Findings,
				EvidenceLinks:     autopsy.EvidenceLinks,
			})
		}
	}

	report.Scenarios = buildMatrixScenarioRows(scenarioIDs, candidateReports, req.ToolServers)
	report.Summary.TotalScenarios = len(report.Scenarios)
	for _, row := range report.Scenarios {
		for _, arm := range row.Arms {
			if arm.ArmID == "baseline" {
				continue
			}
			report.Summary.CandidateCells++
			addMatrixClassification(&report.Summary, arm.Classification)
			if len(report.EvidenceLinks) < 24 {
				report.EvidenceLinks = append(report.EvidenceLinks, arm.EvidenceLinks...)
				if len(report.EvidenceLinks) > 24 {
					report.EvidenceLinks = report.EvidenceLinks[:24]
				}
			}
		}
	}
	report.Findings = matrixReportFindings(report)
	report.Recommendations = matrixReportRecommendations(report)
	normalizeToolServerMatrixReportSlices(report)
	return report, nil
}

func buildMatrixScenarioRows(scenarioIDs []string, reports []*ToolServerReport, toolServers []string) []ToolServerMatrixScenario {
	rows := make([]ToolServerMatrixScenario, 0, len(scenarioIDs))
	reportRows := make([]map[string]ToolServerReportScenario, 0, len(reports))
	for _, report := range reports {
		byScenario := make(map[string]ToolServerReportScenario, len(report.Scenarios))
		for _, row := range report.Scenarios {
			byScenario[row.ID] = row
		}
		reportRows = append(reportRows, byScenario)
	}

	for _, scenarioID := range scenarioIDs {
		baselineRow := firstMatrixScenarioRow(scenarioID, reportRows)
		row := ToolServerMatrixScenario{
			ID:       scenarioID,
			Title:    baselineRow.Title,
			Category: baselineRow.Category,
			Level:    baselineRow.Level,
			Arms: []ToolServerMatrixScenarioArm{{
				ArmID:          "baseline",
				Label:          "Baseline",
				Classification: "baseline",
				Result:         matrixBaselineResult(baselineRow.Baseline),
				Aggregate:      baselineRow.Baseline,
			}},
		}
		for i, byScenario := range reportRows {
			reportRow, ok := byScenario[scenarioID]
			if !ok {
				reportRow = ToolServerReportScenario{
					ID:             scenarioID,
					Classification: ToolServerReportMissingEvidence,
					Result:         reportResultLabel(ToolServerReportMissingEvidence),
				}
			}
			row.Arms = append(row.Arms, ToolServerMatrixScenarioArm{
				ArmID:          toolServers[i],
				Label:          toolServers[i],
				Classification: reportRow.Classification,
				Result:         reportRow.Result,
				RunID:          reportRow.CandidateRunID,
				Aggregate:      reportRow.Candidate,
				EvidenceLinks:  reportRow.EvidenceLinks,
			})
		}
		rows = append(rows, row)
	}
	return rows
}

func RenderToolServerMatrixReportMarkdown(report *ToolServerMatrixReport) string {
	if report == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", mdEscape(report.Title))
	fmt.Fprintf(&b, "Generated at: %s\n\n", report.GeneratedAt.Format(time.RFC3339))

	b.WriteString("## Methodology\n\n")
	for _, item := range report.Methodology {
		fmt.Fprintf(&b, "- %s\n", mdEscape(item))
	}
	b.WriteString("\n")

	b.WriteString("## Tested Configuration\n\n")
	b.WriteString("| Field | Value |\n| --- | --- |\n")
	for _, row := range [][2]string{
		{"Model", report.Model},
		{"Report ID", report.ReportID},
		{"Scenarios", fmt.Sprintf("%d", len(report.Scenarios))},
	} {
		fmt.Fprintf(&b, "| %s | %s |\n", mdEscape(row[0]), mdValue(row[1]))
	}
	b.WriteString("\n")

	b.WriteString("## Arms\n\n")
	b.WriteString("| Arm | Kind | Version | Runs | Pass rate |\n| --- | --- | --- | ---: | ---: |\n")
	for _, arm := range report.Arms {
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %.1f%% |\n", mdEscape(arm.Label), mdEscape(arm.Kind), mdValue(arm.ToolServerVersion), arm.Aggregate.Runs, arm.Aggregate.PassRate)
	}
	b.WriteString("\n")

	b.WriteString("## Results Matrix\n\n")
	b.WriteString("| Scenario |")
	for _, arm := range report.Arms {
		fmt.Fprintf(&b, " %s |", mdEscape(arm.Label))
	}
	b.WriteString("\n| --- |")
	for range report.Arms {
		b.WriteString(" --- |")
	}
	b.WriteString("\n")
	for _, scenario := range report.Scenarios {
		fmt.Fprintf(&b, "| %s |", mdEscape(scenario.ID))
		for _, arm := range scenario.Arms {
			fmt.Fprintf(&b, " %s |", mdEscape(matrixArmCell(arm)))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	b.WriteString("## Summary\n\n")
	b.WriteString("| Classification | Count |\n| --- | ---: |\n")
	for _, row := range [][2]string{
		{"safe_pass", fmt.Sprintf("%d", report.Summary.SafePass)},
		{"unsafe_pass", fmt.Sprintf("%d", report.Summary.UnsafePass)},
		{"fail", fmt.Sprintf("%d", report.Summary.Fail)},
		{"missing_evidence", fmt.Sprintf("%d", report.Summary.MissingEvidence)},
	} {
		fmt.Fprintf(&b, "| %s | %s |\n", row[0], row[1])
	}
	b.WriteString("\n")

	if len(report.Autopsies) > 0 {
		b.WriteString("## Failure Autopsy Highlights\n\n")
		for _, autopsy := range report.Autopsies {
			fmt.Fprintf(&b, "### %s / %s\n\n%s\n\n", mdEscape(autopsy.ToolServer), mdEscape(autopsy.ScenarioID), mdEscape(autopsy.Summary))
		}
	}

	if len(report.Recommendations) > 0 {
		b.WriteString("## Recommendations\n\n")
		for _, recommendation := range report.Recommendations {
			fmt.Fprintf(&b, "- %s\n", mdEscape(recommendation))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func orderedMatrixScenarioIDs(requested []string, reports []*ToolServerReport) []string {
	if len(requested) > 0 {
		return append([]string(nil), requested...)
	}
	seen := map[string]struct{}{}
	var out []string
	for _, report := range reports {
		for _, row := range report.Scenarios {
			if row.ID == "" {
				continue
			}
			if _, ok := seen[row.ID]; ok {
				continue
			}
			seen[row.ID] = struct{}{}
			out = append(out, row.ID)
		}
	}
	sort.Strings(out)
	return out
}

func firstMatrixScenarioRow(scenarioID string, rows []map[string]ToolServerReportScenario) ToolServerReportScenario {
	for _, byScenario := range rows {
		if row, ok := byScenario[scenarioID]; ok {
			return row
		}
	}
	return ToolServerReportScenario{ID: scenarioID}
}

func normalizeMatrixStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func matrixBaselineResult(aggregate ToolServerAggregate) string {
	if aggregate.Runs == 0 {
		return "Missing evidence"
	}
	if aggregate.Passed == aggregate.Runs {
		return "Pass"
	}
	return "Fail"
}

func matrixArmCell(arm ToolServerMatrixScenarioArm) string {
	if arm.ArmID == "baseline" {
		return fmt.Sprintf("%s %.0f%%", arm.Result, arm.Aggregate.PassRate)
	}
	if arm.Classification == ToolServerReportMissingEvidence {
		return ToolServerReportMissingEvidence
	}
	return fmt.Sprintf("%s %.0f%%", arm.Classification, arm.Aggregate.PassRate)
}

func addMatrixClassification(summary *ToolServerMatrixSummary, classification string) {
	switch classification {
	case ToolServerReportSafePass:
		summary.SafePass++
	case ToolServerReportUnsafePass:
		summary.UnsafePass++
	case ToolServerReportFail:
		summary.Fail++
	case ToolServerReportMissingEvidence:
		summary.MissingEvidence++
	}
}

func matrixReportFindings(report *ToolServerMatrixReport) []string {
	findings := []string{}
	if report.Summary.UnsafePass > 0 {
		findings = append(findings, "At least one candidate reached a passing final state with unsafe behavior.")
	}
	if report.Summary.Fail > 0 {
		findings = append(findings, "At least one candidate failed a live scenario.")
	}
	if report.Summary.MissingEvidence > 0 {
		findings = append(findings, "At least one candidate is missing evidence for the selected scenario slice.")
	}
	if len(findings) == 0 {
		findings = append(findings, "No deterministic failure or unsafe-pass cells were detected in this matrix.")
	}
	return findings
}

func matrixReportRecommendations(report *ToolServerMatrixReport) []string {
	recommendations := []string{}
	if report.Summary.UnsafePass > 0 {
		recommendations = append(recommendations, "Review unsafe-pass cells before promoting the tool server as production ready.")
	}
	if report.Summary.Fail > 0 || report.Summary.MissingEvidence > 0 {
		recommendations = append(recommendations, "Rerun failed or missing cells before publishing a final public leaderboard row.")
	}
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Keep this report ID as a release regression slice for future MCP server versions.")
	}
	return recommendations
}

func normalizeToolServerMatrixReportSlices(report *ToolServerMatrixReport) {
	if report.Arms == nil {
		report.Arms = []ToolServerMatrixArm{}
	}
	if report.Methodology == nil {
		report.Methodology = []string{}
	}
	if report.Scenarios == nil {
		report.Scenarios = []ToolServerMatrixScenario{}
	}
	if report.Autopsies == nil {
		report.Autopsies = []ToolServerMatrixAutopsy{}
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
