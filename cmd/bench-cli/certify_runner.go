package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/environment"
	"github.com/vitas/evidra-bench/pkg/harness"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

// runCertifyTrack runs certification for one track and model without printing.
func runCertifyTrack(ctx context.Context, cfg config.Config, track, model string) (*CertResult, error) {
	scenariosDir, err := filepath.Abs(cfg.ScenariosDir)
	if err != nil {
		return nil, fmt.Errorf("resolve scenarios dir: %w", err)
	}
	cfg.ScenariosDir = scenariosDir

	allScenarios, err := scenario.LoadAll(scenariosDir)
	if err != nil {
		return nil, fmt.Errorf("load scenarios: %w", err)
	}

	var trackScenarios []*scenario.Scenario
	for _, s := range allScenarios {
		if s.Track == track {
			trackScenarios = append(trackScenarios, s)
		}
	}
	if len(trackScenarios) == 0 {
		return nil, fmt.Errorf("no scenarios for track %q", track)
	}

	selected, skippedCount := filterRunnableScenarios(trackScenarios, cfg.EnvironmentProvider, io.Discard)
	if len(selected) == 0 {
		return nil, fmt.Errorf("no compatible scenarios for track %q on provider %s", track, cfg.EnvironmentProvider)
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

	// Acquire a batch lease when reusing cluster.
	var batchLease *environment.Lease
	if cfg.ReuseCluster && !cfg.DryRun {
		if err := validateSingleProfile(selected); err != nil {
			return nil, err
		}
		provisioner := newLocalProvisioner(cfg)
		batchLease, err = provisioner.Acquire(ctx, environment.ProvisionRequest{
			Scenario:           selected[0],
			Profile:            selected[0].ResolvedProfile(),
			ProviderName:       cfg.EnvironmentProvider,
			ClusterName:        cfg.ClusterName,
			ReuseCluster:       cfg.ReuseCluster,
			ExistingKubeconfig: cfg.KubeconfigPath,
			Shared:             true,
		})
		if err != nil {
			return nil, fmt.Errorf("certify: acquire batch lease: %w", err)
		}
		defer func() {
			if releaseErr := batchLease.Release(ctx); releaseErr != nil {
				log.Printf("[certify] warning: release batch lease: %v", releaseErr)
			}
		}()
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
		runCfg := prepareScenarioRunConfig(cfg, s, model, runDir)

		if cfg.ReuseCluster && batchLease != nil {
			cleanBenchNamespace(ctx, batchLease.KubeconfigPath, s)
		}

		var prov batchLeaseProvisioner
		if batchLease != nil {
			prov = newLocalProvisioner(runCfg)
		}
		var runResult *harness.RunResult
		var runErr error
		runResult, batchLease, runErr = runWithBatchLeaseRecovery(
			ctx, runCfg, s, batchLease, prov,
			func(l *environment.Lease) (*harness.RunResult, error) {
				return runScenarioOnceWithLease(ctx, runCfg, s, l)
			},
			"certify",
		)

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

	certPath := filepath.Join(outDir, "certification.json")
	cert := &CertResult{
		Track:        track,
		Model:        model,
		Provider:     cfg.Provider,
		Grade:        grade,
		LevelMax:     levelMax,
		Total:        totalCount,
		Passed:       passedCount,
		Skipped:      skippedCount,
		ByLevel:      levelResults,
		Duration:     time.Since(startTime),
		CertifiedAt:  time.Now().UTC(),
		ArtifactPath: certPath,
	}

	certJSON, _ := json.MarshalIndent(cert, "", "  ")
	if err := os.WriteFile(certPath, certJSON, 0o644); err != nil {
		return nil, fmt.Errorf("write certification result: %w", err)
	}

	return cert, nil
}
