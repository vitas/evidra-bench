package signalaudit

import (
	"fmt"
	"sort"
	"strings"
)

// FormatSummary renders a compact human-readable audit summary.
func FormatSummary(result Result) string {
	var b strings.Builder

	fmt.Fprintf(&b, "audited runs: %d\n", result.RunCount)
	fmt.Fprintf(&b, "audited scenarios: %d\n", result.AuditedScenarioCount)
	fmt.Fprintf(&b, "findings: missing_expected=%d forbidden_signals=%d unexpected_extras=%d unstable_groups=%d\n",
		result.FindingTotals.MissingExpected,
		result.FindingTotals.ForbiddenSignals,
		result.FindingTotals.UnexpectedExtras,
		result.FindingTotals.UnstableGroups,
	)

	scenarios := append([]ScenarioFinding(nil), result.ScenarioFindings...)
	sort.Slice(scenarios, func(i, j int) bool {
		left := totalScenarioFindings(scenarios[i])
		right := totalScenarioFindings(scenarios[j])
		if left != right {
			return left > right
		}
		return scenarios[i].ScenarioID < scenarios[j].ScenarioID
	})

	if len(scenarios) == 0 {
		b.WriteString("worst scenarios: none\n")
		return b.String()
	}

	b.WriteString("worst scenarios:\n")
	for _, scenario := range scenarios {
		fmt.Fprintf(&b, "  %s primary=%s runs=%d missing=%d forbidden=%d extra=%d unstable=%d\n",
			scenario.ScenarioID,
			scenario.PrimarySignal,
			scenario.RunCount,
			scenario.MissingExpectedCount,
			scenario.ForbiddenSignalCount,
			scenario.UnexpectedExtraCount,
			scenario.UnstableGroupCount,
		)
	}

	return b.String()
}
