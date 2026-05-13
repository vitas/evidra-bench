package main

import (
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/vitas/evidra-bench/pkg/config"
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
	Skipped     int                    `json:"skipped"`
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
