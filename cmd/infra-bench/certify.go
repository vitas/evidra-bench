package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/scenario"
)

// CertResult holds the certification outcome for a track.
type CertResult struct {
	Track       string                 `json:"track"`
	Model       string                 `json:"model"`
	Provider    string                 `json:"provider"`
	Grade       string                 `json:"grade"`
	LevelMax    string                 `json:"level_max"`
	Total       int                    `json:"total"`
	Passed      int                    `json:"passed"`
	ByLevel     map[string]LevelResult `json:"by_level"`
	Duration    time.Duration          `json:"duration"`
	CertifiedAt time.Time              `json:"certified_at"`
}

// LevelResult holds pass/fail counts for one level.
type LevelResult struct {
	Total  int     `json:"total"`
	Passed int     `json:"passed"`
	Rate   float64 `json:"rate"`
}

var trackNames = map[string]string{
	"k8s-admin":     "Kubernetes Admin",
	"k8s-security":  "Kubernetes Security",
	"release-ops":   "Release Operations",
	"platform-eng":  "Platform Engineering",
	"incident-mgmt": "Incident Management",
}

var levelLabels = map[string]string{
	"L1": "Fix",
	"L2": "Diagnose",
	"L3": "Judge",
	"L4": "Investigate",
}

// orderedLevels defines the level ordering from lowest to highest.
var orderedLevels = []string{"L1", "L2", "L3", "L4"}

func executeCertify(cmd *cobra.Command, cfg config.Config, track, model string) error {
	scenariosDir, err := filepath.Abs(cfg.ScenariosDir)
	if err != nil {
		return fmt.Errorf("resolve scenarios dir: %w", err)
	}
	cfg.ScenariosDir = scenariosDir

	if _, ok := trackNames[track]; !ok {
		valid := make([]string, 0, len(trackNames))
		for k := range trackNames {
			valid = append(valid, k)
		}
		sort.Strings(valid)
		return fmt.Errorf("certify: unknown track %q (valid: %s)", track, strings.Join(valid, ", "))
	}

	if model == "" {
		return fmt.Errorf("certify: --model is required")
	}

	if !cfg.DryRun && cfg.Provider == "" {
		cfg.Provider = "claude"
	}

	// 1. Load all scenarios
	allScenarios, err := scenario.LoadAll(scenariosDir)
	if err != nil {
		return fmt.Errorf("load scenarios: %w", err)
	}

	// 2. Filter by track
	var selected []*scenario.Scenario
	for _, s := range allScenarios {
		if s.Track == track && !s.Skip {
			selected = append(selected, s)
		}
	}
	if len(selected) == 0 {
		return fmt.Errorf("certify: no scenarios found for track %q", track)
	}

	// 3. Sort by level (L1 first, L4 last)
	levelOrder := map[string]int{"L1": 0, "L2": 1, "L3": 2, "L4": 3}
	sort.Slice(selected, func(i, j int) bool {
		li := levelOrder[selected[i].Level]
		lj := levelOrder[selected[j].Level]
		if li != lj {
			return li < lj
		}
		return selected[i].ID < selected[j].ID
	})

	stamp := time.Now().UTC().Format("20060102-150405")
	outDir := filepath.Join(cfg.RunsDir, "certify", fmt.Sprintf("%s_%s_%s", track, model, stamp))
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	// 4. Run each scenario
	startTime := time.Now()
	byLevel := map[string]*LevelResult{}
	totalCount := 0
	passedCount := 0

	for i, s := range selected {
		totalCount++
		level := s.Level
		if level == "" {
			level = "L1"
		}
		if byLevel[level] == nil {
			byLevel[level] = &LevelResult{}
		}
		byLevel[level].Total++

		runDir := filepath.Join(outDir, fmt.Sprintf("%s_%s_r1", s.ID, model))
		evidenceDir := filepath.Join(runDir, "evidence")

		runCfg := cfg
		runCfg.Scenario = s.Path
		runCfg.Model = model
		runCfg.RunsDir = runDir
		runCfg.EvidraEvidenceDir = evidenceDir

		label := fmt.Sprintf("[%d/%d] %s (%s)", i+1, len(selected), s.ID, level)
		fmt.Fprintf(cmd.OutOrStdout(), "%s ...\n", label)

		// Clean namespace between scenarios when reusing cluster.
		if cfg.ReuseCluster {
			cleanBenchNamespace(cmd.Context(), cfg.ClusterName, s)
		}

		runResult, runErr := runScenarioOnce(cmd.Context(), runCfg, s)

		passed := false
		dur := ""
		errMsg := ""
		if runErr != nil {
			errMsg = runErr.Error()
		} else {
			passed = runResult.Passed
			dur = runResult.Duration.Round(time.Millisecond).String()
		}

		if passed {
			passedCount++
			byLevel[level].Passed++
		}

		verdict := "PASS"
		if !passed {
			verdict = "FAIL"
		}
		var rfe *RunFailedError
		if errMsg != "" && !errors.As(runErr, &rfe) {
			verdict = "ERROR"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %s %s %s\n", verdict, dur, errMsg)
	}

	totalDuration := time.Since(startTime)

	// 5. Calculate rates
	levelResults := map[string]LevelResult{}
	for level, lr := range byLevel {
		lr.Rate = float64(lr.Passed) / float64(max(lr.Total, 1))
		levelResults[level] = *lr
	}

	// 6. Determine grade
	grade, levelMax := calculateGrade(levelResults)

	cert := CertResult{
		Track:       track,
		Model:       model,
		Provider:    cfg.Provider,
		Grade:       grade,
		LevelMax:    levelMax,
		Total:       totalCount,
		Passed:      passedCount,
		ByLevel:     levelResults,
		Duration:    totalDuration,
		CertifiedAt: time.Now().UTC(),
	}

	// 7. Print certification output
	printCertification(cmd, cert)

	// 8. Write certification.json
	certPath := filepath.Join(outDir, "certification.json")
	certJSON, err := json.MarshalIndent(cert, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal certification: %w", err)
	}
	if err := os.WriteFile(certPath, certJSON, 0644); err != nil {
		return fmt.Errorf("write certification.json: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\n  Artifacts: %s\n", certPath)

	if passedCount < totalCount {
		return fmt.Errorf("certify: %d/%d scenarios passed", passedCount, totalCount)
	}
	return nil
}

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

	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "════════════════════════════════════════════════════\n")
	fmt.Fprintf(w, "  EVIDRA AGENT CERTIFICATION\n")
	fmt.Fprintf(w, "════════════════════════════════════════════════════\n")
	fmt.Fprintf(w, "  Agent:    %s (%s)\n", cert.Model, cert.Provider)
	fmt.Fprintf(w, "  Track:    %s (%s)\n", trackLabel, cert.Track)
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "  Grade:    %s (%s)\n", strings.ToUpper(cert.Grade), cert.LevelMax)
	fmt.Fprintf(w, "\n")

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
		fmt.Fprintf(w, "  %s %-11s %d/%-3d %s\n", level, label+":", lr.Passed, lr.Total, check)
	}

	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "  Overall:  %d/%d (%.1f%%)\n", cert.Passed, cert.Total, overallRate)
	fmt.Fprintf(w, "  Duration: %s\n", formatDuration(cert.Duration))
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "  Certified: %s\n", cert.CertifiedAt.Format("2006-01-02"))
	fmt.Fprintf(w, "════════════════════════════════════════════════════\n")
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	if m > 0 {
		return fmt.Sprintf("%dm %02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
