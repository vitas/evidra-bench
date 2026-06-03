package benchsvc

import (
	"sort"
	"strings"

	"github.com/vitas/evidra-bench/pkg/autopsy"
)

const (
	failureModeDiagnosis       = "diagnosis"
	failureModeRootCause       = "root_cause"
	failureModePatching        = "patching"
	failureModeVerification    = "verification"
	failureModeSafety          = "safety"
	failureModeToolMisuse      = "tool_misuse"
	failureModeMissingEvidence = "missing_evidence"
	failureModeOther           = "other"
)

// ToolServerMatrixFailureModeBreakdownRow groups candidate failures by derived failure mode.
type ToolServerMatrixFailureModeBreakdownRow struct {
	ArmID            string   `json:"arm_id"`
	ToolServer       string   `json:"tool_server"`
	FailureMode      string   `json:"failure_mode"`
	FailureModeLabel string   `json:"failure_mode_label"`
	UnsafePass       int      `json:"unsafe_pass"`
	Fail             int      `json:"fail"`
	MissingEvidence  int      `json:"missing_evidence"`
	ScenarioIDs      []string `json:"scenario_ids,omitempty"`
}

func deriveFailureMode(classification, primaryFailure string, findings []ToolServerReportFinding) string {
	switch classification {
	case ToolServerReportUnsafePass:
		return failureModeSafety
	case ToolServerReportMissingEvidence:
		return failureModeMissingEvidence
	}

	if mode := failureModeFromText(primaryFailure); mode != "" {
		return mode
	}
	for _, finding := range findings {
		if mode := failureModeFromText(finding.Kind); mode != "" {
			return mode
		}
		if mode := failureModeFromText(finding.Message); mode != "" {
			return mode
		}
	}
	return failureModeOther
}

func failureModeFromText(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return ""
	}

	switch {
	case normalized == string(autopsy.FailureWrongRootCause):
		return failureModeRootCause
	case normalized == string(autopsy.FailureMissedDiagnosticStep),
		normalized == string(autopsy.FailureRetryLoop),
		normalized == string(autopsy.FailureTimeoutNoProgress),
		normalized == string(autopsy.FailureGaveUp):
		return failureModeDiagnosis
	case normalized == string(autopsy.FailurePrematureSuccess):
		return failureModeVerification
	case normalized == string(autopsy.FailureUnsafeAction):
		return failureModeSafety
	case normalized == string(autopsy.FailureIrrelevantAction):
		return failureModePatching
	case normalized == string(autopsy.FailureToolMisuse):
		return failureModeToolMisuse
	case strings.Contains(normalized, "root_cause"), strings.Contains(normalized, "root cause"):
		return failureModeRootCause
	case strings.Contains(normalized, "diagnosis"), strings.Contains(normalized, "diagnose"), strings.Contains(normalized, "investigation"):
		return failureModeDiagnosis
	case strings.Contains(normalized, "patch"), strings.Contains(normalized, "remediation"), strings.Contains(normalized, "fix"):
		return failureModePatching
	case strings.Contains(normalized, "verification"), strings.Contains(normalized, "verify"), strings.Contains(normalized, "postcheck"):
		return failureModeVerification
	case strings.Contains(normalized, "safety"), strings.Contains(normalized, "unsafe"), strings.Contains(normalized, "overbroad"):
		return failureModeSafety
	case strings.Contains(normalized, "tool_misuse"), strings.Contains(normalized, "tool misuse"), strings.Contains(normalized, "fabricated"):
		return failureModeToolMisuse
	default:
		return ""
	}
}

func failureModeLabel(mode string) string {
	switch mode {
	case failureModeDiagnosis:
		return "Diagnosis"
	case failureModeRootCause:
		return "Root cause"
	case failureModePatching:
		return "Patching"
	case failureModeVerification:
		return "Verification"
	case failureModeSafety:
		return "Safety"
	case failureModeToolMisuse:
		return "Tool misuse"
	case failureModeMissingEvidence:
		return "Missing evidence"
	default:
		return "Other"
	}
}

func failureModeSortRank(mode string) int {
	switch mode {
	case failureModeSafety:
		return 0
	case failureModeRootCause:
		return 1
	case failureModeDiagnosis:
		return 2
	case failureModePatching:
		return 3
	case failureModeVerification:
		return 4
	case failureModeToolMisuse:
		return 5
	case failureModeMissingEvidence:
		return 6
	default:
		return 7
	}
}

type failureModeBreakdownAccumulator struct {
	byKey   map[string]*ToolServerMatrixFailureModeBreakdownRow
	seen    map[string]map[string]struct{}
	armRank map[string]int
}

func newFailureModeBreakdownAccumulator() *failureModeBreakdownAccumulator {
	return &failureModeBreakdownAccumulator{
		byKey:   map[string]*ToolServerMatrixFailureModeBreakdownRow{},
		seen:    map[string]map[string]struct{}{},
		armRank: map[string]int{},
	}
}

func (a *failureModeBreakdownAccumulator) add(armID, toolServer, scenarioID, classification, mode string) {
	if mode == "" {
		mode = failureModeOther
	}
	switch classification {
	case ToolServerReportUnsafePass, ToolServerReportFail, ToolServerReportMissingEvidence:
	default:
		return
	}
	if _, ok := a.armRank[armID]; !ok {
		a.armRank[armID] = len(a.armRank)
	}

	key := armID + "\x00" + mode
	row := a.byKey[key]
	if row == nil {
		row = &ToolServerMatrixFailureModeBreakdownRow{
			ArmID:            armID,
			ToolServer:       toolServer,
			FailureMode:      mode,
			FailureModeLabel: failureModeLabel(mode),
		}
		a.byKey[key] = row
		a.seen[key] = map[string]struct{}{}
	}
	if scenarioID != "" {
		if _, ok := a.seen[key][scenarioID]; ok {
			return
		}
		a.seen[key][scenarioID] = struct{}{}
		row.ScenarioIDs = append(row.ScenarioIDs, scenarioID)
		sort.Strings(row.ScenarioIDs)
	}

	switch classification {
	case ToolServerReportUnsafePass:
		row.UnsafePass++
	case ToolServerReportFail:
		row.Fail++
	case ToolServerReportMissingEvidence:
		row.MissingEvidence++
	}
}

func (a *failureModeBreakdownAccumulator) rows() []ToolServerMatrixFailureModeBreakdownRow {
	rows := make([]ToolServerMatrixFailureModeBreakdownRow, 0, len(a.byKey))
	for _, row := range a.byKey {
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ArmID != rows[j].ArmID {
			return a.armRank[rows[i].ArmID] < a.armRank[rows[j].ArmID]
		}
		leftRank := failureModeSortRank(rows[i].FailureMode)
		rightRank := failureModeSortRank(rows[j].FailureMode)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return rows[i].FailureMode < rows[j].FailureMode
	})
	return rows
}
