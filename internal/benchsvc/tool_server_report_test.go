package benchsvc

import (
	"context"
	"strings"
	"testing"
	"time"

	bench "github.com/vitas/evidra-bench/pkg/bench"
)

func TestBuildToolServerReport_ClassifiesRunsAndSurfacesAutopsy(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	repo := &handlerRepo{
		scenarios: []bench.ScenarioSummary{
			{ID: "safe-scenario", Title: "Safe Scenario", Category: "kubernetes", Level: "L2"},
			{ID: "unsafe-scenario", Title: "Unsafe Scenario", Category: "security", Level: "L3"},
			{ID: "fail-scenario", Title: "Fail Scenario", Category: "helm", Level: "L2"},
			{ID: "missing-scenario", Title: "Missing Scenario", Category: "terraform", Level: "L3"},
		},
		runs: []bench.RunRecord{
			reportRun("baseline-safe", "safe-scenario", "", "", true, start),
			reportRun("baseline-unsafe", "unsafe-scenario", "", "", true, start.Add(time.Minute)),
			reportRun("baseline-fail", "fail-scenario", "", "", true, start.Add(2*time.Minute)),
			reportRun("baseline-missing", "missing-scenario", "", "", true, start.Add(3*time.Minute)),
			reportRun("candidate-safe", "safe-scenario", "kubernetes-mcp", "1.2.3", true, start.Add(4*time.Minute)),
			reportRun("candidate-unsafe", "unsafe-scenario", "kubernetes-mcp", "1.2.3", true, start.Add(5*time.Minute)),
			reportRun("candidate-fail", "fail-scenario", "kubernetes-mcp", "1.2.3", false, start.Add(6*time.Minute)),
		},
		artifacts: map[string][]byte{
			"candidate-unsafe:failure_autopsy": []byte(`{
				"outcome":"pass",
				"primary_failure":"unsafe_action",
				"summary":"Final state passed, but the agent used an unsafe action.",
				"findings":[{"kind":"unsafe_action","severity":"critical","message":"Deleted resources outside the target namespace."}]
			}`),
			"candidate-fail:failure_autopsy": []byte(`{
				"outcome":"fail",
				"primary_failure":"retry_loop",
				"summary":"Run failed with repeated diagnostics.",
				"findings":[{"kind":"retry_loop","severity":"warning","message":"Repeated the same command."}]
			}`),
		},
	}
	svc := NewService(repo, ServiceConfig{})

	report, err := svc.BuildToolServerReport(context.Background(), "tenant-a", ToolServerReportRequest{
		Model:             "sonnet",
		ToolServer:        "kubernetes-mcp",
		ToolServerVersion: "1.2.3",
		ScenarioIDs:       []string{"safe-scenario", "unsafe-scenario", "fail-scenario", "missing-scenario"},
	})
	if err != nil {
		t.Fatalf("BuildToolServerReport: %v", err)
	}

	if report.Summary.TotalScenarios != 4 {
		t.Fatalf("total scenarios = %d, want 4", report.Summary.TotalScenarios)
	}
	if report.Summary.SafePass != 1 || report.Summary.UnsafePass != 1 || report.Summary.Fail != 1 || report.Summary.MissingEvidence != 1 {
		t.Fatalf("summary = %+v, want safe=1 unsafe=1 fail=1 missing=1", report.Summary)
	}
	assertScenarioClassification(t, report, "safe-scenario", "safe_pass")
	assertScenarioClassification(t, report, "unsafe-scenario", "unsafe_pass")
	assertScenarioClassification(t, report, "fail-scenario", "fail")
	assertScenarioClassification(t, report, "missing-scenario", "missing_evidence")

	if len(report.Autopsies) != 2 {
		t.Fatalf("autopsies = %d, want 2", len(report.Autopsies))
	}
	if report.Autopsies[0].ScenarioID != "fail-scenario" && report.Autopsies[1].ScenarioID != "fail-scenario" {
		t.Fatalf("autopsies missing fail-scenario: %+v", report.Autopsies)
	}
	if report.Configuration.ToolServer != "kubernetes-mcp" || report.Configuration.ToolServerVersion != "1.2.3" {
		t.Fatalf("configuration = %+v, want selected tool server", report.Configuration)
	}
}

func TestRenderToolServerReportMarkdown_IncludesDeliverableSections(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	repo := &handlerRepo{
		scenarios: []bench.ScenarioSummary{
			{
				ID:                 "fail-scenario",
				Title:              "Fail Scenario",
				Category:           "helm",
				Level:              "L2",
				AutopsyDescription: "Root cause: release is stuck. Safe repair: inspect hooks before rollback.",
			},
		},
		runs: []bench.RunRecord{
			reportRun("baseline-fail", "fail-scenario", "", "", true, start),
			reportRun("candidate-fail", "fail-scenario", "kubernetes-mcp", "1.2.3", false, start.Add(time.Minute)),
		},
		artifacts: map[string][]byte{
			"candidate-fail:failure_autopsy": []byte(`{
				"outcome":"fail",
				"primary_failure":"retry_loop",
				"summary":"Run failed with repeated diagnostics.",
				"findings":[{"kind":"retry_loop","severity":"warning","message":"Repeated the same command."}]
			}`),
		},
	}
	svc := NewService(repo, ServiceConfig{})
	report, err := svc.BuildToolServerReport(context.Background(), "tenant-a", ToolServerReportRequest{
		Model:       "sonnet",
		ToolServer:  "kubernetes-mcp",
		ScenarioIDs: []string{"fail-scenario"},
	})
	if err != nil {
		t.Fatalf("BuildToolServerReport: %v", err)
	}

	md := RenderToolServerReportMarkdown(report)
	for _, want := range []string{
		"# Evidra Bench Tool Server Report",
		"## 1. Executive Summary",
		"## 2. Tested Configuration",
		"## 4. Results Table",
		"## 6. Failure Autopsy",
		"## 10. Raw Evidence Links / Artifacts",
		"| fail-scenario |",
		"Root cause: release is stuck.",
		"Run failed with repeated diagnostics.",
		"/bench/runs/candidate-fail/autopsy",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown missing %q:\n%s", want, md)
		}
	}
}

func reportRun(id, scenarioID, toolServer, toolServerVersion string, passed bool, createdAt time.Time) bench.RunRecord {
	return bench.RunRecord{
		ID:                id,
		TenantID:          "tenant-a",
		ScenarioID:        scenarioID,
		Model:             "sonnet",
		Provider:          "bifrost",
		ToolServer:        toolServer,
		ToolServerVersion: toolServerVersion,
		Passed:            passed,
		Duration:          30,
		Turns:             10,
		PromptTokens:      1000,
		CompletionTokens:  500,
		EstimatedCost:     0.01,
		ChecksPassed:      boolToInt(passed),
		ChecksTotal:       1,
		CreatedAt:         createdAt,
	}
}

func assertScenarioClassification(t *testing.T, report *ToolServerReport, scenarioID, want string) {
	t.Helper()
	for _, row := range report.Scenarios {
		if row.ID == scenarioID {
			if row.Classification != want {
				t.Fatalf("%s classification = %q, want %q", scenarioID, row.Classification, want)
			}
			return
		}
	}
	t.Fatalf("scenario %q not found in report", scenarioID)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
