package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"samebits.com/evidra-infra-bench/pkg/config"
)

// executeCertifyRace runs certification for multiple models in parallel and prints a race result.
func executeCertifyRace(cmd *cobra.Command, cfg config.Config, track string, models []string) error {
	w := cmd.OutOrStdout()
	trackLabel := trackNames[track]
	if trackLabel == "" {
		trackLabel = track
	}

	writef(w, "\n")
	writef(w, "🏁 CERTIFICATION RACE: %s\n", trackLabel)
	writef(w, "   Contenders: %s\n", strings.Join(models, " vs "))
	writef(w, "════════════════════════════════════════════════════\n\n")

	type raceResult struct {
		model string
		cert  *CertResult
		err   error
	}

	results := make(chan raceResult, len(models))

	for _, m := range models {
		go func(model string) {
			// Each model gets its own config with unique cluster to avoid conflicts.
			raceCfg := cfg
			raceCfg.ClusterName = fmt.Sprintf("%s-%s", cfg.ClusterName, strings.ReplaceAll(model, "/", "-"))

			cert, err := runCertifySingle(cmd.Context(), raceCfg, track, model)
			results <- raceResult{model: model, cert: cert, err: err}
		}(m)
	}

	// Collect results
	var certs []raceResult
	for range models {
		certs = append(certs, <-results)
	}

	// Sort by: grade (expert > proficient > competent > novice), then pass rate, then duration.
	gradeOrder := map[string]int{"expert": 4, "proficient": 3, "competent": 2, "novice": 1}
	sort.Slice(certs, func(i, j int) bool {
		if certs[i].cert == nil {
			return false
		}
		if certs[j].cert == nil {
			return true
		}
		gi := gradeOrder[certs[i].cert.Grade]
		gj := gradeOrder[certs[j].cert.Grade]
		if gi != gj {
			return gi > gj
		}
		ri := float64(certs[i].cert.Passed) / float64(max(certs[i].cert.Total, 1))
		rj := float64(certs[j].cert.Passed) / float64(max(certs[j].cert.Total, 1))
		if ri != rj {
			return ri > rj
		}
		return certs[i].cert.Duration < certs[j].cert.Duration
	})

	// Print race results
	writef(w, "\n")
	writef(w, "🏁 RACE RESULTS: %s\n", trackLabel)
	writef(w, "════════════════════════════════════════════════════\n")

	for i, r := range certs {
		medal := "  "
		switch i {
		case 0:
			medal = "🥇"
		case 1:
			medal = "🥈"
		case 2:
			medal = "🥉"
		}

		if r.err != nil && r.cert == nil {
			writef(w, "  %s %-25s ERROR: %v\n", medal, r.model, r.err)
			continue
		}
		c := r.cert
		rate := float64(c.Passed) / float64(max(c.Total, 1)) * 100
		writef(w, "  %s %-25s %s (%s)  %d/%d (%.0f%%)  %s\n",
			medal, c.Model, strings.ToUpper(c.Grade), c.LevelMax,
			c.Passed, c.Total, rate, formatDuration(c.Duration))
	}
	writef(w, "════════════════════════════════════════════════════\n")

	return nil
}
