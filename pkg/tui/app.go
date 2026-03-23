// Package tui provides an interactive terminal UI for browsing and running scenarios.
package tui

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/harness"
	"samebits.com/evidra-infra-bench/pkg/scenario"
)

// view modes
const (
	viewCatalog = iota
	viewRunning
	viewResult
	viewConfig
	viewHelp
	viewHistory
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

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case RunFinishedMsg:
		a.runResult = msg.Result
		a.runErr = msg.Err
		a.view = viewResult
		a.refreshHistory()
		a.applyFilter()
		return a, nil

	case tea.KeyMsg:
		return a.handleKey(msg)
	}
	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global keys
	if key == "ctrl+c" {
		return a, tea.Quit
	}

	switch a.view {
	case viewHelp:
		a.view = viewCatalog
		return a, nil
	case viewResult:
		a.view = viewCatalog
		return a, nil
	case viewHistory:
		a.view = viewCatalog
		return a, nil
	case viewConfig:
		return a.handleConfigKey(msg)
	case viewRunning:
		return a, nil
	}

	// Catalog view
	if a.filtering {
		return a.handleFilterKey(msg)
	}

	switch key {
	case "q":
		return a, tea.Quit
	case "j", "down":
		if a.cursor < len(a.filtered)-1 {
			a.cursor++
		}
	case "k", "up":
		if a.cursor > 0 {
			a.cursor--
		}
	case "g", "home":
		a.cursor = 0
	case "G", "end":
		if len(a.filtered) > 0 {
			a.cursor = len(a.filtered) - 1
		}
	case "/":
		a.filtering = true
		a.query = ""
	case "t":
		a.categoryIdx = (a.categoryIdx + 1) % len(CategoryFilters)
		a.applyFilter()
	case "d":
		a.cfg.DryRun = !a.cfg.DryRun
		_ = SaveLabConfig(a.cfgPath, a.cfg)
	case "e":
		a.view = viewConfig
	case "p":
		a.cycleProvider()
		_ = SaveLabConfig(a.cfgPath, a.cfg)
	case "m":
		a.cycleModel()
		_ = SaveLabConfig(a.cfgPath, a.cfg)
	case "h":
		if len(a.filtered) > 0 {
			a.view = viewHistory
		}
	case "?":
		a.view = viewHelp
	case "enter":
		if len(a.filtered) > 0 {
			return a, a.runScenario()
		}
	}
	return a, nil
}

func (a *App) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "enter", "esc":
		a.filtering = false
	case "backspace":
		if len(a.query) > 0 {
			a.query = a.query[:len(a.query)-1]
			a.applyFilter()
		}
	default:
		if len(key) == 1 {
			a.query += key
			a.applyFilter()
		}
	}
	return a, nil
}

func (a *App) handleConfigKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc", "e", "q":
		_ = SaveLabConfig(a.cfgPath, a.cfg)
		a.view = viewCatalog
	case "1":
		if a.cfg.Adapter == "cli" {
			a.cfg.Adapter = "mcp"
		} else {
			a.cfg.Adapter = "cli"
		}
	case "2":
		a.cfg.DryRun = !a.cfg.DryRun
	case "3":
		a.cycleModel()
	case "4":
		a.cycleProvider()
	}
	return a, nil
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

// ProviderChoices are the available providers for cycling.
var ProviderChoices = []string{"", "bifrost", "claude"}

func (a *App) cycleProvider() {
	idx := 0
	for i, p := range ProviderChoices {
		if p == a.cfg.Provider {
			idx = i
			break
		}
	}
	a.cfg.Provider = ProviderChoices[(idx+1)%len(ProviderChoices)]
}

// ModelChoices are the available models for cycling.
var ModelChoices = []string{"", "sonnet", "haiku", "opus"}

func (a *App) cycleModel() {
	idx := 0
	for i, m := range ModelChoices {
		if m == a.cfg.Model {
			idx = i
			break
		}
	}
	a.cfg.Model = ModelChoices[(idx+1)%len(ModelChoices)]
}

func (a *App) applyFilter() {
	cat := ""
	if a.categoryIdx < len(CategoryFilters) {
		cat = CategoryFilters[a.categoryIdx]
	}
	a.filtered = FilterCatalog(a.allItems, a.query, cat)
	if a.cursor >= len(a.filtered) {
		a.cursor = max(0, len(a.filtered)-1)
	}
}

func (a *App) runScenario() tea.Cmd {
	s := a.filtered[a.cursor].Scenario
	a.view = viewRunning
	a.runOutput = fmt.Sprintf("Running scenario: %s ...\n", s.ID)
	a.runResult = nil
	a.runErr = nil

	return func() tea.Msg {
		cfg := config.Config{
			Scenario:          s.ID,
			ScenariosDir:      a.scenariosDir,
			Adapter:           a.cfg.Adapter,
			Provider:          a.cfg.Provider,
			AgentCommand:      a.cfg.AgentCommand,
			Model:             a.cfg.Model,
			EvidraBin:         a.cfg.EvidraBin,
			Timeout:           a.cfg.TimeoutDuration(),
			DryRun:            a.cfg.DryRun,
			RunsDir:           a.runsDir,
			ClusterName:       "infra-bench",
			EvidraEvidenceDir: a.cfg.EvidraEvidenceDir,
			ProxyMode:         a.cfg.ProxyMode,
			SmartPrescribe:    a.cfg.SmartPrescribe,
			EvidraURL:         a.cfg.EvidraURL,
			EvidraAPIKey:      a.cfg.EvidraAPIKey,
			MemoryWindow:      a.cfg.MemoryWindow,
			ReuseCluster:      a.cfg.ReuseCluster,
		}

		if a.cfg.DryRun {
			log.SetOutput(os.Stderr)
			h := harness.New(a.harnessDeps)
			result, err := h.Run(context.Background(), harness.RunRequest{
				Config:   cfg,
				Scenario: s,
			})
			return RunFinishedMsg{Result: result, Err: err}
		}

		if agentCommandRequired(a.cfg) {
			return RunFinishedMsg{Err: fmt.Errorf("agent command not set — press 'e' to configure")}
		}

		h := harness.New(a.harnessDeps)
		result, err := h.Run(context.Background(), harness.RunRequest{
			Config:   cfg,
			Scenario: s,
		})
		return RunFinishedMsg{Result: result, Err: err}
	}
}

func agentCommandRequired(cfg LabConfig) bool {
	return cfg.Provider == "" && cfg.AgentCommand == ""
}

func (a *App) View() string {
	switch a.view {
	case viewHelp:
		return a.renderHelp()
	case viewRunning:
		return a.renderRunning()
	case viewResult:
		return a.renderResult()
	case viewConfig:
		return a.renderConfig()
	case viewHistory:
		return a.renderHistory()
	default:
		return a.renderCatalog()
	}
}

func (a *App) renderCatalog() string {
	var b strings.Builder

	// Title bar
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	title := "infra-bench lab"
	if a.dbTotal > 0 {
		title += fmt.Sprintf("  %d runs %s", a.dbTotal, a.dbPassRate)
	}
	b.WriteString(titleStyle.Render(title))

	catFilter := CategoryFilters[a.categoryIdx]
	if catFilter == "" {
		catFilter = "all"
	}
	filterInfo := fmt.Sprintf("  [%s]", catFilter)
	if a.query != "" {
		filterInfo += fmt.Sprintf("  /%s", a.query)
	}
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	b.WriteString(dimStyle.Render(filterInfo))

	if a.cfg.Provider != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  [%s]", a.cfg.Provider)))
	}
	if a.cfg.Model != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  [%s]", a.cfg.Model)))
	}
	if a.cfg.DryRun {
		b.WriteString(dimStyle.Render("  [dry-run]"))
	}
	b.WriteString("\n\n")

	// Catalog list
	if len(a.filtered) == 0 {
		b.WriteString(dimStyle.Render("  No scenarios match filter\n"))
	}

	visibleStart, visibleEnd := a.visibleRange()
	for i := visibleStart; i < visibleEnd; i++ {
		item := a.filtered[i]
		cursor := "  "
		if i == a.cursor {
			cursor = "> "
		}

		badge := " "
		switch item.LastResult {
		case "pass":
			badge = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("P")
		case "fail":
			badge = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("F")
		}

		evidraMark := " "
		if item.Scenario.Evidra.Enabled {
			evidraMark = lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render("E")
		}

		catStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Width(12)
		idStyle := lipgloss.NewStyle().Width(30)
		if i == a.cursor {
			idStyle = idStyle.Bold(true).Foreground(lipgloss.Color("86"))
		}

		runCount := ""
		if stats, ok := a.statsMap[item.Scenario.ID]; ok && stats.TotalRuns > 0 {
			runCount = dimStyle.Render(fmt.Sprintf(" (%d/%d)", stats.PassCount, stats.TotalRuns))
		}

		line := fmt.Sprintf("%s%s %s %s %s%s",
			cursor,
			badge,
			evidraMark,
			catStyle.Render(strings.Join(item.Scenario.ResolvedCategories(), "/")),
			idStyle.Render(item.Scenario.ID),
			runCount,
		)
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Detail pane for selected item
	if len(a.filtered) > 0 && a.cursor < len(a.filtered) {
		b.WriteString("\n")
		b.WriteString(a.renderDetail(a.filtered[a.cursor]))
	}

	// Bottom bar
	b.WriteString("\n")
	if a.filtering {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("filter: "))
		b.WriteString(a.query)
		b.WriteString("_")
	} else {
		b.WriteString(dimStyle.Render("j/k:nav  /:filter  t:cat  p:provider  m:model  h:history  d:dry-run  e:config  enter:run  ?:help  q:quit"))
	}

	return b.String()
}

func (a *App) renderDetail(item CatalogItem) string {
	var b strings.Builder
	s := item.Scenario

	headerStyle := lipgloss.NewStyle().Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	b.WriteString(headerStyle.Render(s.Title))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("  category: %s  timeout: %s", strings.Join(s.ResolvedCategories(), "/"), s.Timeout.Duration)))
	b.WriteString("\n")

	if len(s.Tags) > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  tags: %s", strings.Join(s.Tags, ", "))))
		b.WriteString("\n")
	}

	if len(s.Checks) > 0 {
		checks := make([]string, len(s.Checks))
		for i, c := range s.Checks {
			checks[i] = c.Type
			if c.Name != "" {
				checks[i] += "/" + c.Name
			}
		}
		b.WriteString(dimStyle.Render(fmt.Sprintf("  checks: %s", strings.Join(checks, ", "))))
		b.WriteString("\n")
	}

	if s.Evidra.Enabled {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render("  evidra: enabled"))
		if s.Evidra.ExpectedRiskLevel != "" {
			b.WriteString(dimStyle.Render(fmt.Sprintf("  risk: %s", s.Evidra.ExpectedRiskLevel)))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (a *App) renderRunning() string {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")).Render(a.runOutput)
}

func (a *App) renderResult() string {
	var b strings.Builder
	if a.runErr != nil {
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196")).Render("ERROR"))
		b.WriteString("\n\n")
		b.WriteString(a.runErr.Error())
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Press any key to return"))
		return b.String()
	}

	r := a.runResult
	verdictStyle := lipgloss.NewStyle().Bold(true)
	if r.Passed {
		b.WriteString(verdictStyle.Foreground(lipgloss.Color("42")).Render("PASS"))
	} else {
		b.WriteString(verdictStyle.Foreground(lipgloss.Color("196")).Render("FAIL"))
	}

	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	b.WriteString(dimStyle.Render(fmt.Sprintf("  scenario=%s  duration=%s", r.ScenarioID, r.Duration.Round(time.Millisecond))))
	b.WriteString("\n\n")

	if r.Checks != nil {
		for _, c := range r.Checks.Checks {
			icon := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("ok")
			if c.Verdict == "fail" {
				icon = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("!!")
			}
			b.WriteString(fmt.Sprintf("  %s %s", icon, c.Name))
			if c.Message != "" {
				b.WriteString(dimStyle.Render(fmt.Sprintf(" — %s", c.Message)))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Press any key to return"))
	return b.String()
}

func (a *App) renderConfig() string {
	var b strings.Builder
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	b.WriteString(headerStyle.Render("Run Configuration"))
	b.WriteString("\n\n")

	providerDisplay := a.cfg.Provider
	if providerDisplay == "" {
		providerDisplay = "(none — uses adapter)"
	}
	modelDisplay := a.cfg.Model
	if modelDisplay == "" {
		modelDisplay = "(default)"
	}
	b.WriteString(fmt.Sprintf("  [1] Adapter:       %s\n", a.cfg.Adapter))
	b.WriteString(fmt.Sprintf("  [2] Dry-run:       %v\n", a.cfg.DryRun))
	b.WriteString(fmt.Sprintf("  [3] Model:         %s\n", modelDisplay))
	b.WriteString(fmt.Sprintf("  [4] Provider:      %s\n", providerDisplay))
	b.WriteString(fmt.Sprintf("      Agent command: %s\n", a.cfg.AgentCommand))
	b.WriteString(fmt.Sprintf("      Evidra bin:    %s\n", a.cfg.EvidraBin))
	b.WriteString(fmt.Sprintf("      Timeout:       %s\n", a.cfg.Timeout))
	if a.cfg.EvidraEvidenceDir != "" {
		b.WriteString(fmt.Sprintf("      Evidence dir:  %s\n", a.cfg.EvidraEvidenceDir))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("1/2/3/4: toggle  esc: back"))
	return b.String()
}

func (a *App) renderHelp() string {
	var b strings.Builder
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	b.WriteString(headerStyle.Render("infra-bench lab — Help"))
	b.WriteString("\n\n")
	b.WriteString("  j/k, arrows   Navigate scenario list\n")
	b.WriteString("  /             Search by text (id, title, tags)\n")
	b.WriteString("  t             Cycle category filter (all/kubernetes/helm/argocd)\n")
	b.WriteString("  Enter         Run selected scenario\n")
	b.WriteString("  p             Cycle provider (bifrost/claude/none)\n")
	b.WriteString("  m             Cycle model (sonnet/haiku/opus/default)\n")
	b.WriteString("  h             Show run history for selected scenario\n")
	b.WriteString("  d             Toggle dry-run mode\n")
	b.WriteString("  e             Edit run configuration\n")
	b.WriteString("  ?             Show this help\n")
	b.WriteString("  q             Quit\n")
	b.WriteString("\n")
	b.WriteString("  Badges:\n")
	b.WriteString(fmt.Sprintf("  %s = last run passed\n", lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("P")))
	b.WriteString(fmt.Sprintf("  %s = last run failed\n", lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("F")))
	b.WriteString(fmt.Sprintf("  %s = evidra protocol checks enabled\n", lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render("E")))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Press any key to return"))
	return b.String()
}

func (a *App) renderHistory() string {
	var b strings.Builder
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	if len(a.filtered) == 0 || a.cursor >= len(a.filtered) {
		b.WriteString(dimStyle.Render("No scenario selected"))
		return b.String()
	}

	s := a.filtered[a.cursor].Scenario
	runs := HistoryForScenario(a.history, s.ID)
	stats := ComputeStats(runs)

	b.WriteString(headerStyle.Render(fmt.Sprintf("Run History: %s", s.ID)))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("  Total runs: %d   Pass: %d   Fail: %d\n\n",
		stats.TotalRuns, stats.PassCount, stats.FailCount))

	if len(runs) == 0 {
		b.WriteString(dimStyle.Render("  No runs yet\n"))
	}

	maxShow := 10
	if len(runs) < maxShow {
		maxShow = len(runs)
	}
	for i := 0; i < maxShow; i++ {
		r := runs[i]
		verdict := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("PASS")
		if !r.Passed {
			verdict = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("FAIL")
		}
		ts := r.StartTime.Format("2006-01-02 15:04:05")
		dur := r.Duration().Round(time.Millisecond)
		checkSummary := ""
		if r.Checks != nil {
			passCount := 0
			for _, c := range r.Checks.Checks {
				if c.Verdict == "pass" {
					passCount++
				}
			}
			checkSummary = dimStyle.Render(fmt.Sprintf("  checks: %d/%d", passCount, len(r.Checks.Checks)))
		}

		// Signal and score info
		signalInfo := ""
		if active := ActiveSignals(r.Signals); len(active) > 0 {
			signalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
			signalInfo = signalStyle.Render(fmt.Sprintf("  signals: %s", strings.Join(active, ", ")))
		}
		scoreInfo := ""
		if r.ScoreBand != "" && r.ScoreBand != "insufficient_data" {
			scoreInfo = dimStyle.Render(fmt.Sprintf("  score: %.0f (%s)", r.Score, r.ScoreBand))
		}

		b.WriteString(fmt.Sprintf("  %s  %s  %s%s%s%s\n", verdict, ts, dimStyle.Render(dur.String()), checkSummary, signalInfo, scoreInfo))

		// Show check diff between this run and the previous one
		if i < maxShow-1 && i+1 < len(runs) {
			diff := checkDiff(runs[i+1], r)
			if diff != "" {
				b.WriteString(dimStyle.Render(diff))
				b.WriteString("\n")
			}
		}
	}

	if len(runs) > maxShow {
		b.WriteString(dimStyle.Render(fmt.Sprintf("\n  ... and %d more runs\n", len(runs)-maxShow)))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Press any key to return"))
	return b.String()
}

// checkDiff compares two runs and returns a summary of what changed.
func checkDiff(prev, curr RunRecord) string {
	if prev.Checks == nil || curr.Checks == nil {
		return ""
	}
	prevMap := make(map[string]string)
	for _, c := range prev.Checks.Checks {
		prevMap[c.Name] = string(c.Verdict)
	}
	var changes []string
	for _, c := range curr.Checks.Checks {
		prevVerdict, existed := prevMap[c.Name]
		if !existed {
			changes = append(changes, fmt.Sprintf("    + %s: %s (new)", c.Name, c.Verdict))
		} else if prevVerdict != string(c.Verdict) {
			changes = append(changes, fmt.Sprintf("    ~ %s: %s -> %s", c.Name, prevVerdict, c.Verdict))
		}
	}
	if len(changes) == 0 {
		return ""
	}
	return strings.Join(changes, "\n")
}

func (a *App) visibleRange() (int, int) {
	maxVisible := a.height - 15 // reserve space for title, detail, status bar
	if maxVisible < 5 {
		maxVisible = 5
	}
	if maxVisible > len(a.filtered) {
		maxVisible = len(a.filtered)
	}
	start := 0
	if a.cursor >= maxVisible {
		start = a.cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(a.filtered) {
		end = len(a.filtered)
	}
	return start, end
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
