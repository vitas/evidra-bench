package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	"workloads":        "Workloads & Scheduling",
	"troubleshooting":  "Troubleshooting",
	"networking":       "Services & Networking",
	"storage":          "Storage",
	"pod-security":     "Pod Security",
	"runtime-security": "Runtime Security",
	"release-ops":      "Release Operations",
	"platform-eng":     "Platform Engineering",
}

// examTracks maps exam shortcuts to their component tracks.
var examTracks = map[string][]string{
	"cka": {"workloads", "troubleshooting", "networking", "storage"},
	"cks": {"pod-security", "runtime-security"},
	"all": {"workloads", "troubleshooting", "networking", "storage", "pod-security", "runtime-security", "release-ops", "platform-eng"},
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
	// Expand exam shortcuts: cka, cks, all → run multiple tracks sequentially.
	if tracks, ok := examTracks[track]; ok {
		return executeCertifyExam(cmd, cfg, track, tracks, model)
	}

	// Support racing multiple models: --model sonnet,gpt-4o
	models := strings.Split(model, ",")
	for i := range models {
		models[i] = strings.TrimSpace(models[i])
	}
	if len(models) > 1 {
		return executeCertifyRace(cmd, cfg, track, models)
	}
	return executeCertifySingle(cmd, cfg, track, model)
}

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

// runCertifySingle runs certification for one model and returns the result (no printing).
func runCertifySingle(ctx context.Context, cfg config.Config, track, model string) (*CertResult, error) {
	scenariosDir, err := filepath.Abs(cfg.ScenariosDir)
	if err != nil {
		return nil, fmt.Errorf("resolve scenarios dir: %w", err)
	}
	cfg.ScenariosDir = scenariosDir

	allScenarios, err := scenario.LoadAll(scenariosDir)
	if err != nil {
		return nil, fmt.Errorf("load scenarios: %w", err)
	}

	var selected []*scenario.Scenario
	for _, s := range allScenarios {
		if s.Track == track && !s.Skip {
			selected = append(selected, s)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no scenarios for track %q", track)
	}

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
		return nil, fmt.Errorf("create certify output dir: %w", err)
	}

	startTime := time.Now()
	byLevel := map[string]*LevelResult{}
	totalCount, passedCount := 0, 0

	for _, s := range selected {
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

		if cfg.ReuseCluster {
			cleanBenchNamespace(ctx, cfg.ClusterName, s)
		}

		runResult, runErr := runScenarioOnce(ctx, runCfg, s)

		passed := false
		if runErr == nil {
			passed = runResult.Passed
		}
		if passed {
			passedCount++
			byLevel[level].Passed++
		}
	}

	levelResults := map[string]LevelResult{}
	for level, lr := range byLevel {
		lr.Rate = float64(lr.Passed) / float64(max(lr.Total, 1))
		levelResults[level] = *lr
	}
	grade, levelMax := calculateGrade(levelResults)

	cert := &CertResult{
		Track:       track,
		Model:       model,
		Provider:    cfg.Provider,
		Grade:       grade,
		LevelMax:    levelMax,
		Total:       totalCount,
		Passed:      passedCount,
		ByLevel:     levelResults,
		Duration:    time.Since(startTime),
		CertifiedAt: time.Now().UTC(),
	}

	certJSON, _ := json.MarshalIndent(cert, "", "  ")
	if err := os.WriteFile(filepath.Join(outDir, "certification.json"), certJSON, 0o644); err != nil {
		return nil, fmt.Errorf("write certification result: %w", err)
	}

	return cert, nil
}

func executeCertifySingle(cmd *cobra.Command, cfg config.Config, track, model string) error {
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
		writef(cmd.OutOrStdout(), "%s ...\n", label)

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
		writef(cmd.OutOrStdout(), "  %s %s %s\n", verdict, dur, errMsg)
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
	writef(cmd.OutOrStdout(), "\n  Artifacts: %s\n", certPath)

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

func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
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
