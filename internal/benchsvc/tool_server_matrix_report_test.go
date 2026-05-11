package benchsvc

import (
	"context"
	"strings"
	"testing"
	"time"

	bench "samebits.com/evidra-infra-bench/pkg/bench"
)

func TestBuildToolServerMatrixReport(t *testing.T) {
	t.Parallel()

	svc := NewService(matrixReportRepo(), ServiceConfig{})
	report, err := svc.BuildToolServerMatrixReport(context.Background(), "tenant-a", ToolServerMatrixReportRequest{
		Model:              "sonnet",
		ReportID:           "public-report",
		ToolServers:        []string{"flux159-mcp-server-kubernetes", "containers-kubernetes-mcp-server"},
		ToolServerVersions: []string{"1.0.0", "2.0.0"},
		ScenarioIDs:        []string{"s1", "s2", "s3", "s4"},
	})
	if err != nil {
		t.Fatalf("BuildToolServerMatrixReport: %v", err)
	}

	if !strings.Contains(report.Title, "Kubernetes MCP Server Readiness Report") {
		t.Fatalf("title = %q, want Kubernetes MCP Server Readiness Report", report.Title)
	}
	if report.ReportID != "public-report" {
		t.Fatalf("report_id = %q, want public-report", report.ReportID)
	}
	if len(report.Arms) != 3 {
		t.Fatalf("arms = %+v, want baseline plus two candidates", report.Arms)
	}
	if report.Arms[0].ID != "baseline" {
		t.Fatalf("first arm = %+v, want baseline", report.Arms[0])
	}
	if report.Arms[1].ToolServer != "flux159-mcp-server-kubernetes" || report.Arms[2].ToolServer != "containers-kubernetes-mcp-server" {
		t.Fatalf("candidate arms = %+v, want selected tool servers", report.Arms)
	}
	if len(report.Scenarios) != 4 {
		t.Fatalf("scenario rows = %d, want 4", len(report.Scenarios))
	}
	for _, row := range report.Scenarios {
		if len(row.Arms) != 3 {
			t.Fatalf("scenario %s arms = %+v, want three-arm row", row.ID, row.Arms)
		}
	}
	if report.Summary.SafePass != 5 || report.Summary.UnsafePass != 1 || report.Summary.Fail != 1 || report.Summary.MissingEvidence != 1 {
		t.Fatalf("summary = %+v, want safe=5 unsafe=1 fail=1 missing=1", report.Summary)
	}
	assertMatrixScenarioArm(t, report, "s2", "flux159-mcp-server-kubernetes", ToolServerReportUnsafePass)
	assertMatrixScenarioArm(t, report, "s3", "flux159-mcp-server-kubernetes", ToolServerReportFail)
	assertMatrixScenarioArm(t, report, "s4", "flux159-mcp-server-kubernetes", ToolServerReportMissingEvidence)
}

func TestRenderToolServerMatrixReportMarkdown(t *testing.T) {
	t.Parallel()

	svc := NewService(matrixReportRepo(), ServiceConfig{})
	report, err := svc.BuildToolServerMatrixReport(context.Background(), "tenant-a", ToolServerMatrixReportRequest{
		Model:              "sonnet",
		ReportID:           "public-report",
		ToolServers:        []string{"flux159-mcp-server-kubernetes", "containers-kubernetes-mcp-server"},
		ToolServerVersions: []string{"1.0.0", "2.0.0"},
		ScenarioIDs:        []string{"s1", "s2", "s3", "s4"},
	})
	if err != nil {
		t.Fatalf("BuildToolServerMatrixReport: %v", err)
	}

	md := RenderToolServerMatrixReportMarkdown(report)
	for _, want := range []string{
		"# Kubernetes MCP Server Readiness Report",
		"## Methodology",
		"| Scenario | Baseline | flux159-mcp-server-kubernetes | containers-kubernetes-mcp-server |",
		"unsafe_pass",
		"missing_evidence",
		"Used an unsafe broad action.",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown missing %q:\n%s", want, md)
		}
	}
}

func matrixReportRepo() *handlerRepo {
	start := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	reportID := "public-report"
	return &handlerRepo{
		scenarios: []bench.ScenarioSummary{
			{ID: "s1", Title: "Safe scenario", Category: "kubernetes", Level: "L2"},
			{ID: "s2", Title: "Unsafe scenario", Category: "kubernetes", Level: "L3"},
			{ID: "s3", Title: "Failed scenario", Category: "kubernetes", Level: "L2"},
			{ID: "s4", Title: "Missing scenario", Category: "kubernetes", Level: "L3"},
		},
		runs: []bench.RunRecord{
			matrixReportRun("baseline-s1", "s1", "", "", true, reportID, start),
			matrixReportRun("baseline-s2", "s2", "", "", true, reportID, start),
			matrixReportRun("baseline-s3", "s3", "", "", true, reportID, start),
			matrixReportRun("baseline-s4", "s4", "", "", true, reportID, start),
			matrixReportRun("flux-s1", "s1", "flux159-mcp-server-kubernetes", "1.0.0", true, reportID, start.Add(time.Minute)),
			matrixReportRun("flux-s2", "s2", "flux159-mcp-server-kubernetes", "1.0.0", true, reportID, start.Add(2*time.Minute)),
			matrixReportRun("flux-s3", "s3", "flux159-mcp-server-kubernetes", "1.0.0", false, reportID, start.Add(3*time.Minute)),
			matrixReportRun("containers-s1", "s1", "containers-kubernetes-mcp-server", "2.0.0", true, reportID, start.Add(time.Minute)),
			matrixReportRun("containers-s2", "s2", "containers-kubernetes-mcp-server", "2.0.0", true, reportID, start.Add(2*time.Minute)),
			matrixReportRun("containers-s3", "s3", "containers-kubernetes-mcp-server", "2.0.0", true, reportID, start.Add(3*time.Minute)),
			matrixReportRun("containers-s4", "s4", "containers-kubernetes-mcp-server", "2.0.0", true, reportID, start.Add(4*time.Minute)),
			matrixReportRun("other-report-flux-s1", "s1", "flux159-mcp-server-kubernetes", "1.0.0", false, "other-report", start.Add(5*time.Minute)),
		},
		artifacts: map[string][]byte{
			"flux-s2:failure_autopsy": []byte(`{"primary_failure":"unsafe_action","summary":"Used an unsafe broad action.","findings":[{"kind":"unsafe_action","severity":"high","message":"Patched a broad selector."}]}`),
		},
	}
}

func matrixReportRun(id, scenarioID, toolServer, toolServerVersion string, passed bool, reportID string, createdAt time.Time) bench.RunRecord {
	run := reportRun(id, scenarioID, toolServer, toolServerVersion, passed, createdAt)
	run.MetadataJSON = `{"report_id":"` + reportID + `"}`
	return run
}

func assertMatrixScenarioArm(t *testing.T, report *ToolServerMatrixReport, scenarioID, armID, wantClassification string) {
	t.Helper()
	for _, row := range report.Scenarios {
		if row.ID != scenarioID {
			continue
		}
		for _, arm := range row.Arms {
			if arm.ArmID == armID {
				if arm.Classification != wantClassification {
					t.Fatalf("%s/%s classification = %q, want %q", scenarioID, armID, arm.Classification, wantClassification)
				}
				return
			}
		}
		t.Fatalf("arm %q not found in scenario %q: %+v", armID, scenarioID, row.Arms)
	}
	t.Fatalf("scenario %q not found", scenarioID)
}
