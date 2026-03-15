package signalaudit

// Manifest maps scenario IDs to signal-audit expectations.
type Manifest map[string]Expectation

// Result is the machine-readable output of a signal audit.
type Result struct {
	RunCount             int                  `json:"run_count"`
	AuditedScenarioCount int                  `json:"audited_scenario_count"`
	RunFindings          []RunFinding         `json:"run_findings,omitempty"`
	ScenarioFindings     []ScenarioFinding    `json:"scenario_findings,omitempty"`
	InstabilityFindings  []InstabilityFinding `json:"instability_findings,omitempty"`
	FindingTotals        FindingTotals        `json:"finding_totals"`
}

// FindingTotals summarizes the number of findings by type.
type FindingTotals struct {
	MissingExpected  int `json:"missing_expected"`
	ForbiddenSignals int `json:"forbidden_signals"`
	UnexpectedExtras int `json:"unexpected_extras"`
	UnstableGroups   int `json:"unstable_groups"`
}

// Run is the subset of a run artifact needed for signal auditing.
type Run struct {
	RunDir       string
	ScenarioID   string
	Model        string
	Provider     string
	Signals      []string
	SignalCounts map[string]int
	SignalSource string
}

// RunFinding describes one audited run with its signal mismatches.
type RunFinding struct {
	RunDir           string   `json:"run_dir"`
	ScenarioID       string   `json:"scenario_id"`
	Model            string   `json:"model,omitempty"`
	Provider         string   `json:"provider,omitempty"`
	SignalSource     string   `json:"signal_source,omitempty"`
	ObservedSignals  []string `json:"observed_signals,omitempty"`
	MissingExpected  []string `json:"missing_expected,omitempty"`
	ForbiddenSignals []string `json:"forbidden_signals,omitempty"`
	UnexpectedExtras []string `json:"unexpected_extras,omitempty"`
}

// ScenarioFinding aggregates findings for one scenario across audited runs.
type ScenarioFinding struct {
	ScenarioID           string `json:"scenario_id"`
	PrimarySignal        string `json:"primary_signal"`
	RunCount             int    `json:"run_count"`
	MissingExpectedCount int    `json:"missing_expected_count"`
	ForbiddenSignalCount int    `json:"forbidden_signal_count"`
	UnexpectedExtraCount int    `json:"unexpected_extra_count"`
	UnstableGroupCount   int    `json:"unstable_group_count"`
}

// InstabilityFinding describes a repeated run group whose observed signal sets differ.
type InstabilityFinding struct {
	ScenarioID         string   `json:"scenario_id"`
	Model              string   `json:"model,omitempty"`
	Provider           string   `json:"provider,omitempty"`
	RunDirs            []string `json:"run_dirs,omitempty"`
	ObservedSignalSets []string `json:"observed_signal_sets,omitempty"`
}

// Expectation defines which signals a scenario should or should not emit.
type Expectation struct {
	PrimarySignal           string   `yaml:"primary_signal"`
	ExpectedSignals         []string `yaml:"expected_signals,omitempty"`
	AllowedSecondarySignals []string `yaml:"allowed_secondary_signals,omitempty"`
	ForbiddenSignals        []string `yaml:"forbidden_signals,omitempty"`
}
