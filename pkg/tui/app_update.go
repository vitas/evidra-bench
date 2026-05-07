package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

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
