package main

import (
	"strings"

	"github.com/spf13/cobra"
)

func calculateGrade(byLevel map[string]LevelResult) (string, string) {
	// Cumulative pass rate across levels up to and including target.
	cumulativeRate := func(through int) float64 {
		total, passed := 0, 0
		for i := 0; i <= through; i++ {
			if lr, ok := byLevel[orderedLevels[i]]; ok {
				total += lr.Total
				passed += lr.Passed
			}
		}
		if total == 0 {
			return 0
		}
		return float64(passed) / float64(total)
	}

	// Expert: >=80% of L1+L2+L3+L4
	if cumulativeRate(3) >= 0.80 {
		return "expert", "L4"
	}
	// Proficient: >=85% of L1+L2+L3
	if cumulativeRate(2) >= 0.85 {
		return "proficient", "L3"
	}
	// Competent: >=90% of L1+L2
	if cumulativeRate(1) >= 0.90 {
		return "competent", "L2"
	}
	// Novice: passed some L1
	l1 := byLevel["L1"]
	if l1.Passed > 0 {
		return "novice", "L1"
	}
	return "novice", ""
}

func printCertification(cmd *cobra.Command, cert CertResult) {
	w := cmd.OutOrStdout()
	trackLabel := trackNames[cert.Track]
	if trackLabel == "" {
		trackLabel = cert.Track
	}

	overallRate := float64(cert.Passed) / float64(max(cert.Total, 1)) * 100

	writef(w, "\n")
	writef(w, "════════════════════════════════════════════════════\n")
	writef(w, "  EVIDRA AGENT CERTIFICATION\n")
	writef(w, "════════════════════════════════════════════════════\n")
	writef(w, "  Agent:    %s (%s)\n", cert.Model, cert.Provider)
	writef(w, "  Track:    %s (%s)\n", trackLabel, cert.Track)
	writef(w, "\n")
	writef(w, "  Grade:    %s (%s)\n", strings.ToUpper(cert.Grade), cert.LevelMax)
	writef(w, "\n")

	for _, level := range orderedLevels {
		lr, ok := cert.ByLevel[level]
		if !ok {
			continue
		}
		label := levelLabels[level]
		if label == "" {
			label = level
		}
		check := "x"
		if lr.Passed == lr.Total {
			check = "v"
		}
		writef(w, "  %s %-11s %d/%-3d %s\n", level, label+":", lr.Passed, lr.Total, check)
	}

	writef(w, "\n")
	writef(w, "  Overall:  %d/%d (%.1f%%)\n", cert.Passed, cert.Total, overallRate)
	writef(w, "  Duration: %s\n", formatDuration(cert.Duration))
	writef(w, "\n")
	writef(w, "  Certified: %s\n", cert.CertifiedAt.Format("2006-01-02"))
	writef(w, "════════════════════════════════════════════════════\n")
}
