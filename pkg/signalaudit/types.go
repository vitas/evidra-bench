package signalaudit

// Manifest maps scenario IDs to signal-audit expectations.
type Manifest map[string]Expectation

// Expectation defines which signals a scenario should or should not emit.
type Expectation struct {
	PrimarySignal            string   `yaml:"primary_signal"`
	ExpectedSignals          []string `yaml:"expected_signals,omitempty"`
	AllowedSecondarySignals  []string `yaml:"allowed_secondary_signals,omitempty"`
	ForbiddenSignals         []string `yaml:"forbidden_signals,omitempty"`
}
