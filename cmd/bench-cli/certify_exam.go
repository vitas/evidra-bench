package main

import (
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/vitas/evidra-bench/pkg/config"
)

// executeCertifyExam runs certification across multiple tracks (CKA, CKS, or all).
func executeCertifyExam(cmd *cobra.Command, cfg config.Config, examName string, tracks []string, model string) error {
	w := cmd.OutOrStdout()
	examLabel := strings.ToUpper(examName)

	writef(w, "\n")
	writef(w, "════════════════════════════════════════════════════\n")
	writef(w, "  %s CERTIFICATION EXAM\n", examLabel)
	writef(w, "  Agent: %s | Tracks: %d\n", model, len(tracks))
	writef(w, "════════════════════════════════════════════════════\n\n")

	var results []CertResult
	var totalPassed, totalCount int
	startTime := time.Now()

	for _, track := range tracks {
		writef(w, "── Track: %s ──\n", trackNames[track])
		cert, err := runCertifySingle(cmd.Context(), cfg, track, model)
		if err != nil {
			writef(w, "  ERROR: %v\n\n", err)
			continue
		}
		results = append(results, *cert)
		totalPassed += cert.Passed
		totalCount += cert.Total

		check := "✗"
		if cert.Passed == cert.Total {
			check = "✓"
		}
		writef(w, "  %s  %d/%d  %s\n\n", strings.ToUpper(cert.Grade), cert.Passed, cert.Total, check)
	}

	totalDuration := time.Since(startTime)
	overallRate := float64(totalPassed) / float64(max(totalCount, 1)) * 100

	writef(w, "════════════════════════════════════════════════════\n")
	writef(w, "  %s EXAM RESULTS\n", examLabel)
	writef(w, "════════════════════════════════════════════════════\n")
	writef(w, "  Agent:    %s (%s)\n", model, cfg.Provider)
	writef(w, "\n")

	for _, cert := range results {
		trackLabel := trackNames[cert.Track]
		check := "✗"
		if cert.Passed == cert.Total {
			check = "✓"
		}
		writef(w, "  %-25s %-12s %d/%-3d %s\n", trackLabel, strings.ToUpper(cert.Grade), cert.Passed, cert.Total, check)
	}

	writef(w, "\n")
	writef(w, "  Overall:  %d/%d (%.1f%%)\n", totalPassed, totalCount, overallRate)
	writef(w, "  Duration: %s\n", formatDuration(totalDuration))
	writef(w, "════════════════════════════════════════════════════\n")

	return nil
}
