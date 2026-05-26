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

	case reviewUploadMsg:
		if msg.Err != nil {
			a.artifactStatus = "upload failed: " + msg.Err.Error()
		} else {
			a.artifactStatus = "uploaded run review"
		}
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
		if key == "a" && a.runResult != nil && a.openArtifactsForDir(a.runResult.ArtifactDir) {
			return a, nil
		}
		a.view = viewCatalog
		return a, nil
	case viewHistory:
		a.view = viewCatalog
		return a, nil
	case viewArtifact:
		return a.handleArtifactKey(msg)
	case viewReviewEditor:
		return a.handleReviewEditorKey(msg)
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
	case "a":
		a.openLatestArtifactsForSelectedScenario()
	case "?":
		a.view = viewHelp
	case "enter":
		if len(a.filtered) > 0 {
			return a, a.runScenario()
		}
	}
	return a, nil
}

func (a *App) handleArtifactKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc", "q", "backspace":
		a.view = viewCatalog
	case "r":
		a.beginReviewEditor()
	case "right", "l", "tab":
		a.artifactTab = (a.artifactTab + 1) % len(artifactTabs)
	case "left", "h", "shift+tab":
		a.artifactTab--
		if a.artifactTab < 0 {
			a.artifactTab = len(artifactTabs) - 1
		}
	}
	return a, nil
}

func (a *App) handleReviewEditorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if a.artifacts == nil {
		a.view = viewArtifact
		return a, nil
	}
	if a.reviewEditor.EditingNote {
		switch key {
		case "enter":
			a.reviewEditor.EditingNote = false
		case "esc":
			a.reviewEditor.EditingNote = false
		case "backspace":
			if len(a.reviewEditor.Note) > 0 {
				a.reviewEditor.Note = a.reviewEditor.Note[:len(a.reviewEditor.Note)-1]
				a.reviewEditor.NoteDirty = true
			}
		default:
			if len(key) == 1 {
				a.reviewEditor.Note += key
				a.reviewEditor.NoteDirty = true
			}
		}
		return a, nil
	}
	switch key {
	case "esc", "q":
		a.view = viewArtifact
	case "j", "down":
		a.reviewEditor.moveStep(*a.artifacts, 1)
	case "k", "up":
		a.reviewEditor.moveStep(*a.artifacts, -1)
	case "v":
		a.reviewEditor.cycleVerdict(1)
	case "l":
		a.reviewEditor.cycleLabelKind(*a.artifacts, 1)
	case "s":
		a.reviewEditor.cycleSeverity(1)
	case "p":
		a.reviewEditor.cycleVisibility(1)
	case "n":
		a.reviewEditor.EditingNote = true
	case "w":
		return a, a.saveReviewFromEditor(false)
	case "u":
		return a, a.saveReviewFromEditor(true)
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
		a.cycleAdapter()
	case "2":
		a.cfg.DryRun = !a.cfg.DryRun
	case "3":
		a.cycleModel()
	case "4":
		a.cycleProvider()
	}
	return a, nil
}

// AdapterChoices are the available adapters for cycling.
var AdapterChoices = []string{"cli", "mcp", "a2a"}

func (a *App) cycleAdapter() {
	idx := 0
	for i, adapter := range AdapterChoices {
		if adapter == a.cfg.Adapter {
			idx = i
			break
		}
	}
	a.cfg.Adapter = AdapterChoices[(idx+1)%len(AdapterChoices)]
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
