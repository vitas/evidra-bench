package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/environment"
	"samebits.com/evidra-infra-bench/pkg/harness"
	"samebits.com/evidra-infra-bench/pkg/scenario"
)

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

	// 2. Filter by track, then by skip + provider compatibility.
	var trackScenarios []*scenario.Scenario
	for _, s := range allScenarios {
		if s.Track == track {
			trackScenarios = append(trackScenarios, s)
		}
	}
	if len(trackScenarios) == 0 {
		return fmt.Errorf("certify: no scenarios found for track %q", track)
	}

	selected, skippedCount := filterRunnableScenarios(trackScenarios, cfg.EnvironmentProvider, cmd.OutOrStdout())
	if len(selected) == 0 {
		return fmt.Errorf("certify: no compatible scenarios for track %q on provider %s", track, cfg.EnvironmentProvider)
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

	// Acquire a batch lease when reusing cluster.
	var batchLease *environment.Lease
	if cfg.ReuseCluster && !cfg.DryRun {
		if err := validateSingleProfile(selected); err != nil {
			return err
		}
		provisioner := newLocalProvisioner(cfg)
		batchLease, err = provisioner.Acquire(cmd.Context(), environment.ProvisionRequest{
			Scenario:           selected[0],
			Profile:            selected[0].ResolvedProfile(),
			ProviderName:       cfg.EnvironmentProvider,
			ClusterName:        cfg.ClusterName,
			ReuseCluster:       cfg.ReuseCluster,
			ExistingKubeconfig: cfg.KubeconfigPath,
			Shared:             true,
		})
		if err != nil {
			return fmt.Errorf("certify: acquire batch lease: %w", err)
		}
		defer func() {
			if releaseErr := batchLease.Release(cmd.Context()); releaseErr != nil {
				log.Printf("[certify] warning: release batch lease: %v", releaseErr)
			}
		}()
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
		runCfg.EvidenceDir = evidenceDir

		label := fmt.Sprintf("[%d/%d] %s (%s)", i+1, len(selected), s.ID, level)
		writef(cmd.OutOrStdout(), "%s ...\n", label)

		// Clean namespace between scenarios when reusing cluster.
		if cfg.ReuseCluster && batchLease != nil {
			cleanBenchNamespace(cmd.Context(), cfg.ClusterName, s)
		}

		var prov batchLeaseProvisioner
		if batchLease != nil {
			prov = newLocalProvisioner(runCfg)
		}
		var runResult *harness.RunResult
		var runErr error
		runResult, batchLease, runErr = runWithBatchLeaseRecovery(
			cmd.Context(), runCfg, s, batchLease, prov,
			func(l *environment.Lease) (*harness.RunResult, error) {
				return runScenarioOnceWithLease(cmd.Context(), runCfg, s, l)
			},
			"certify",
		)

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
		Skipped:     skippedCount,
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
