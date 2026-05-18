// Package tui provides an interactive terminal UI for browsing and running scenarios.
package tui

import (
	"fmt"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vitas/evidra-bench/pkg/harness"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

// view modes
const (
	viewCatalog = iota
	viewRunning
	viewResult
	viewConfig
	viewHelp
	viewHistory
	viewArtifact
)

// RunFinishedMsg is sent when a scenario run completes.
type RunFinishedMsg struct {
	Result *harness.RunResult
	Err    error
}

// App is the top-level Bubble Tea model.
type App struct {
	allItems     []CatalogItem
	filtered     []CatalogItem
	cursor       int
	query        string
	filtering    bool
	categoryIdx  int
	cfg          LabConfig
	cfgPath      string
	scenariosDir string
	view         int
	width        int
	height       int
	runOutput    string
	runResult    *harness.RunResult
	runErr       error
	harnessDeps  harness.Deps
	runsDir      string
	history      []RunRecord
	statsMap     map[string]ScenarioStats
	dbTotal      int
	dbPassRate   string
	artifacts    *RunArtifacts
	artifactTab  int
}

// NewApp creates a new TUI app.
func NewApp(scenariosDir, cfgPath string, cfg LabConfig, deps harness.Deps) (*App, error) {
	absDir, err := filepath.Abs(scenariosDir)
	if err != nil {
		return nil, fmt.Errorf("tui.NewApp: %w", err)
	}
	scenarios, err := scenario.LoadAll(absDir)
	if err != nil {
		return nil, fmt.Errorf("tui.NewApp: %w", err)
	}
	items := BuildCatalog(scenarios)
	runsDir := cfg.RunsDir
	if runsDir == "" {
		runsDir = "runs"
	}
	app := &App{
		allItems:     items,
		filtered:     items,
		cfg:          cfg,
		cfgPath:      cfgPath,
		scenariosDir: absDir,
		view:         viewCatalog,
		harnessDeps:  deps,
		runsDir:      runsDir,
	}
	app.refreshHistory()
	return app, nil
}

func (a *App) Init() tea.Cmd {
	return nil
}

func (a *App) refreshHistory() {
	a.history = LoadHistory(a.runsDir)
	a.statsMap = make(map[string]ScenarioStats)
	for _, item := range a.allItems {
		scenarioRuns := HistoryForScenario(a.history, item.Scenario.ID)
		stats := ComputeStats(scenarioRuns)
		a.statsMap[item.Scenario.ID] = stats
	}
	for i := range a.allItems {
		if stats, ok := a.statsMap[a.allItems[i].Scenario.ID]; ok {
			a.allItems[i].LastResult = stats.LastResult
		}
	}
	// Compute global stats for title bar
	totalRuns := 0
	totalPass := 0
	for _, stats := range a.statsMap {
		totalRuns += stats.TotalRuns
		totalPass += stats.PassCount
	}
	a.dbTotal = totalRuns
	if totalRuns > 0 {
		a.dbPassRate = fmt.Sprintf("%.0f%%", float64(totalPass)/float64(totalRuns)*100)
	} else {
		a.dbPassRate = ""
	}
}

// Run starts the TUI application.
func Run(scenariosDir, cfgPath string, cfg LabConfig, deps harness.Deps) error {
	app, err := NewApp(scenariosDir, cfgPath, cfg, deps)
	if err != nil {
		return err
	}
	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
