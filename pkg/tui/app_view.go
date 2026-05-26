package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

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
	case viewArtifact:
		return a.renderArtifact()
	case viewReviewEditor:
		return a.renderReviewEditor()
	default:
		return a.renderCatalog()
	}
}

func (a *App) renderCatalog() string {
	var b strings.Builder

	// Title bar
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	title := "bench-cli lab"
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

		catStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Width(12)
		idStyle := lipgloss.NewStyle().Width(30)
		if i == a.cursor {
			idStyle = idStyle.Bold(true).Foreground(lipgloss.Color("86"))
		}

		runCount := ""
		if stats, ok := a.statsMap[item.Scenario.ID]; ok && stats.TotalRuns > 0 {
			runCount = dimStyle.Render(fmt.Sprintf(" (%d/%d)", stats.PassCount, stats.TotalRuns))
		}

		line := fmt.Sprintf("%s%s %s %s%s",
			cursor,
			badge,
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
		b.WriteString(dimStyle.Render("j/k:nav  /:filter  t:cat  p:provider  m:model  h:history  a:artifacts  d:dry-run  e:config  enter:run  ?:help  q:quit"))
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
			fmt.Fprintf(&b, "  %s %s", icon, c.Name)
			if c.Message != "" {
				b.WriteString(dimStyle.Render(fmt.Sprintf(" — %s", c.Message)))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if r.ArtifactDir != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("artifacts: %s\n", r.ArtifactDir)))
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("a:artifacts  any other key:return"))
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
	fmt.Fprintf(&b, "  [1] Adapter:       %s\n", a.cfg.Adapter)
	fmt.Fprintf(&b, "  [2] Dry-run:       %v\n", a.cfg.DryRun)
	fmt.Fprintf(&b, "  [3] Model:         %s\n", modelDisplay)
	fmt.Fprintf(&b, "  [4] Provider:      %s\n", providerDisplay)
	fmt.Fprintf(&b, "      Agent command: %s\n", a.cfg.AgentCommand)
	fmt.Fprintf(&b, "      Timeout:       %s\n", a.cfg.Timeout)
	if a.cfg.A2AAgentURL != "" {
		fmt.Fprintf(&b, "      A2A URL:       %s\n", a.cfg.A2AAgentURL)
	}
	if a.cfg.MCPServer != "" {
		fmt.Fprintf(&b, "      MCP server:    %s\n", a.cfg.MCPServer)
	}
	if toolServer := formatIdentityVersion(a.cfg.ToolServerID, a.cfg.ToolServerVersion); toolServer != "" {
		fmt.Fprintf(&b, "      Tool server:   %s\n", toolServer)
	}
	if skill := formatIdentityVersion(a.cfg.SkillID, a.cfg.SkillVersion); skill != "" {
		fmt.Fprintf(&b, "      Skill:         %s\n", skill)
	}
	if a.cfg.SkillFile != "" {
		fmt.Fprintf(&b, "      Skill file:    %s\n", a.cfg.SkillFile)
	}
	if a.cfg.SystemPromptFile != "" {
		fmt.Fprintf(&b, "      System prompt: %s\n", a.cfg.SystemPromptFile)
	}
	if a.cfg.ReportID != "" {
		fmt.Fprintf(&b, "      Report ID:     %s\n", a.cfg.ReportID)
	}
	if a.cfg.ContractVersion != "" {
		fmt.Fprintf(&b, "      Contract:      %s\n", a.cfg.ContractVersion)
	}
	if a.cfg.ClusterName != "" {
		fmt.Fprintf(&b, "      Cluster:       %s\n", a.cfg.ClusterName)
	}
	if a.cfg.EnvironmentProvider != "" {
		fmt.Fprintf(&b, "      Environment:   %s\n", a.cfg.EnvironmentProvider)
	}
	fmt.Fprintf(&b, "      Memory window: %d\n", a.cfg.MemoryWindow)
	fmt.Fprintf(&b, "      Reuse cluster: %v\n", a.cfg.ReuseCluster)
	if a.cfg.Parallel > 0 {
		fmt.Fprintf(&b, "      Parallel:      %d\n", a.cfg.Parallel)
	}
	if a.cfg.DatabaseURL != "" {
		fmt.Fprintf(&b, "      Database URL:  set\n")
	}
	if a.cfg.BenchURL != "" {
		fmt.Fprintf(&b, "      Bench URL:     %s\n", a.cfg.BenchURL)
	}
	if a.cfg.BenchAPIKey != "" {
		fmt.Fprintf(&b, "      Bench API key: set\n")
	}
	if a.cfg.EvidenceDir != "" {
		fmt.Fprintf(&b, "      Evidence dir:  %s\n", a.cfg.EvidenceDir)
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("1:adapter  2:dry-run  3:model  4:provider  esc:back"))
	return b.String()
}

func formatIdentityVersion(id, version string) string {
	if id == "" {
		return ""
	}
	if version == "" {
		return id
	}
	return id + " @ " + version
}

func (a *App) renderHelp() string {
	var b strings.Builder
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	b.WriteString(headerStyle.Render("bench-cli lab — Help"))
	b.WriteString("\n\n")
	b.WriteString("  j/k, arrows   Navigate scenario list\n")
	b.WriteString("  /             Search by text (id, title, tags)\n")
	b.WriteString("  t             Cycle category filter\n")
	b.WriteString("  Enter         Run selected scenario\n")
	b.WriteString("  p             Cycle provider (bifrost/claude/none)\n")
	b.WriteString("  m             Cycle model (sonnet/haiku/opus/default)\n")
	b.WriteString("  h             Show run history for selected scenario\n")
	b.WriteString("  a             Show latest run artifacts for selected scenario\n")
	b.WriteString("  r             Review the loaded run from artifact view\n")
	b.WriteString("  d             Toggle dry-run mode\n")
	b.WriteString("  e             Review/edit run configuration toggles\n")
	b.WriteString("  ?             Show this help\n")
	b.WriteString("  q             Quit\n")
	b.WriteString("\n")
	b.WriteString("  Badges:\n")
	fmt.Fprintf(&b, "  %s = last run passed\n", lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("P"))
	fmt.Fprintf(&b, "  %s = last run failed\n", lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("F"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Press any key to return"))
	return b.String()
}

func (a *App) renderArtifact() string {
	if a.artifacts == nil {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("No artifacts loaded\n\nesc: back")
	}
	if a.artifactTab < 0 || a.artifactTab >= len(artifactTabs) {
		a.artifactTab = 0
	}
	active := artifactTabs[a.artifactTab]
	var b strings.Builder
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	b.WriteString(headerStyle.Render("Run Artifacts"))
	b.WriteString(dimStyle.Render(fmt.Sprintf("  %s", a.artifacts.Dir)))
	b.WriteString("\n")
	for i, tab := range artifactTabs {
		label := string(tab)
		if !a.artifacts.Has(tab) {
			label += "*"
		}
		style := dimStyle
		if i == a.artifactTab {
			style = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
		}
		b.WriteString(style.Render("[" + label + "]"))
		if i < len(artifactTabs)-1 {
			b.WriteString(" ")
		}
	}
	b.WriteString("\n\n")
	b.WriteString(a.artifacts.Render(active))
	b.WriteString("\n")
	if a.artifactStatus != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("\n" + a.artifactStatus))
		b.WriteString("\n")
	}
	b.WriteString(dimStyle.Render("\nleft/right:tab  r:review  esc/q:back  * missing artifact"))
	return b.String()
}

func (a *App) renderReviewEditor() string {
	if a.artifacts == nil {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("No artifacts loaded\n\nesc: back")
	}
	var b strings.Builder
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	b.WriteString(headerStyle.Render("Run Review Editor"))
	b.WriteString(dimStyle.Render(fmt.Sprintf("  %s", a.artifacts.Dir)))
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "  run:        %s\n", valueOrDefault(a.artifacts.RunID, "(unknown)"))
	fmt.Fprintf(&b, "  scenario:   %s\n", valueOrDefault(a.artifacts.ScenarioID, "(unknown)"))
	fmt.Fprintf(&b, "  verdict:    %s\n", a.reviewEditor.verdict())
	fmt.Fprintf(&b, "  label:      %s\n", a.reviewEditor.labelKind())
	fmt.Fprintf(&b, "  severity:   %s\n", a.reviewEditor.severity())
	fmt.Fprintf(&b, "  visibility: %s\n", a.reviewEditor.visibility())

	if step, ok := selectedReviewStep(*a.artifacts, a.reviewEditor); ok {
		fmt.Fprintf(&b, "\nStep %d", step.Index+1)
		if step.Phase != "" {
			fmt.Fprintf(&b, "  %s", step.Phase)
		}
		b.WriteString("\n")
		fmt.Fprintf(&b, "  evidence: %s\n", evidenceSnippetForStep(step))
		if step.Command != "" && step.Command != evidenceSnippetForStep(step) {
			fmt.Fprintf(&b, "  command:  %s\n", step.Command)
		}
	} else {
		b.WriteString("\nNo timeline steps available for review.\n")
	}

	notePrompt := "note"
	if a.reviewEditor.EditingNote {
		notePrompt = "note (editing)"
	}
	fmt.Fprintf(&b, "\n  %s: %s", notePrompt, a.reviewEditor.Note)
	if a.reviewEditor.EditingNote {
		b.WriteString("_")
	}
	b.WriteString("\n")
	if a.reviewEditor.Status != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("\n" + a.reviewEditor.Status))
		b.WriteString("\n")
	}
	b.WriteString(dimStyle.Render("\nj/k:step  v:verdict  l:label  s:severity  p:visibility  n:note  w:save  u:save+upload  esc:back"))
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

	fmt.Fprintf(&b, "  Total runs: %d   Pass: %d   Fail: %d\n\n",
		stats.TotalRuns, stats.PassCount, stats.FailCount)

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

		fmt.Fprintf(&b, "  %s  %s  %s%s%s%s\n", verdict, ts, dimStyle.Render(dur.String()), checkSummary, signalInfo, scoreInfo)

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
