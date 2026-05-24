package artifactaudit

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// WriteJSON writes an artifact coverage result as indented JSON.
func WriteJSON(path string, result Result) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// FormatSummary renders a compact human-readable coverage summary.
func FormatSummary(result Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "artifact coverage: %d/%d complete (%.1f%%)\n", result.CompleteRuns, result.TotalRuns, result.CoveragePercent)
	formatCounts(&b, "missing by artifact", result.MissingByArtifact)
	formatCounts(&b, "invalid by artifact", result.InvalidByArtifact)
	formatCounts(&b, "mismatch by artifact", result.MismatchByArtifact)
	formatCounts(&b, "missing by adapter", result.MissingByAdapter)
	if len(result.Findings) > 0 {
		b.WriteString("worst gaps:\n")
		limit := len(result.Findings)
		if limit > 10 {
			limit = 10
		}
		for i := 0; i < limit; i++ {
			f := result.Findings[i]
			fmt.Fprintf(&b, "  %s: %s %s (%s)\n", f.RunID, f.Kind, f.Artifact, f.Message)
		}
	}
	return b.String()
}

func formatCounts(b *strings.Builder, title string, counts map[string]int) {
	if len(counts) == 0 {
		return
	}
	b.WriteString(title + ":\n")
	for _, key := range sortedKeys(counts) {
		fmt.Fprintf(b, "  %s: %d\n", key, counts[key])
	}
}
