package signalaudit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vitas/evidra-bench/pkg/signalaudit"
)

func TestWriteJSON_WritesStableReport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "signal-audit.json")
	result := signalaudit.Result{
		RunCount:             2,
		AuditedScenarioCount: 1,
		RunFindings: []signalaudit.RunFinding{
			{
				RunDir:           "/runs/r1",
				ScenarioID:       "broken-deployment",
				Model:            "sonnet",
				Provider:         "claude",
				ObservedSignals:  []string{"blast_radius", "retry_loop"},
				ForbiddenSignals: []string{"blast_radius"},
			},
		},
		FindingTotals: signalaudit.FindingTotals{
			MissingExpected:  0,
			ForbiddenSignals: 1,
			UnexpectedExtras: 0,
			UnstableGroups:   0,
		},
	}

	if err := signalaudit.WriteJSON(path, result); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "\"run_count\": 2") {
		t.Fatalf("json missing run_count: %s", text)
	}
	if !strings.Contains(text, "\"scenario_id\": \"broken-deployment\"") {
		t.Fatalf("json missing scenario finding: %s", text)
	}
	if !strings.Contains(text, "\"forbidden_signals\": 1") {
		t.Fatalf("json missing totals: %s", text)
	}
}

func TestFormatSummary_IncludesFindingTotals(t *testing.T) {
	out := signalaudit.FormatSummary(signalaudit.Result{
		RunCount:             5,
		AuditedScenarioCount: 3,
		FindingTotals: signalaudit.FindingTotals{
			MissingExpected:  1,
			ForbiddenSignals: 2,
			UnexpectedExtras: 3,
			UnstableGroups:   1,
		},
	})
	if !strings.Contains(out, "audited runs: 5") {
		t.Fatalf("summary missing run count: %s", out)
	}
	if !strings.Contains(out, "audited scenarios: 3") {
		t.Fatalf("summary missing scenario count: %s", out)
	}
	if !strings.Contains(out, "missing_expected=1") {
		t.Fatalf("summary missing finding totals: %s", out)
	}
}

func TestFormatSummary_SortsWorstScenariosFirst(t *testing.T) {
	out := signalaudit.FormatSummary(signalaudit.Result{
		RunCount:             4,
		AuditedScenarioCount: 2,
		ScenarioFindings: []signalaudit.ScenarioFinding{
			{
				ScenarioID:           "wrong-probes",
				PrimarySignal:        "blast_radius",
				RunCount:             2,
				UnexpectedExtraCount: 1,
			},
			{
				ScenarioID:           "broken-deployment",
				PrimarySignal:        "retry_loop",
				RunCount:             2,
				MissingExpectedCount: 1,
				ForbiddenSignalCount: 2,
			},
		},
	})

	brokenIndex := strings.Index(out, "broken-deployment")
	wrongIndex := strings.Index(out, "wrong-probes")
	if brokenIndex == -1 || wrongIndex == -1 {
		t.Fatalf("summary missing scenarios: %s", out)
	}
	if brokenIndex > wrongIndex {
		t.Fatalf("worst scenario not listed first: %s", out)
	}
}
