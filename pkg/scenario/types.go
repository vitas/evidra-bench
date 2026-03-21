// Package scenario defines the scenario model and loader.
package scenario

import "time"

// Scenario is the parsed representation of a scenario.yaml file.
type Scenario struct {
	ID          string             `yaml:"id"`
	Title       string             `yaml:"title"`
	Description string             `yaml:"description,omitempty"`
	Category    string             `yaml:"category"`
	Track       string             `yaml:"track,omitempty"` // workloads, troubleshooting, networking, storage, pod-security, runtime-security, release-ops, platform-eng
	Level       string             `yaml:"level,omitempty"` // L1 (fix), L2 (diagnose), L3 (judge), L4 (investigate)
	Path        string             `yaml:"-"`
	Dir         string             `yaml:"-"`
	Tags        []string           `yaml:"tags,omitempty"`
	Prompt      string             `yaml:"prompt"`
	Timeout     Duration           `yaml:"timeout,omitempty"`
	Checks      []Check            `yaml:"checks"`
	Scope       Scope              `yaml:"scope,omitempty"`
	Bootstrap   []BootstrapStep    `yaml:"bootstrap,omitempty"`
	AfterBreak  []BootstrapStep    `yaml:"after_break,omitempty"`
	Break       Break              `yaml:"break"`
	Stages      []Stage            `yaml:"stages,omitempty"`
	Chaos       ChaosConfig        `yaml:"chaos,omitempty"`
	Baseline    string             `yaml:"baseline,omitempty"`
	Tools       []string           `yaml:"tools,omitempty"`
	Evidra      EvidraExpectations `yaml:"evidra,omitempty"`
	Skip        bool               `yaml:"skip,omitempty"`
	SkipReason  string             `yaml:"skip_reason,omitempty"`
}

// Check describes a verification assertion.
type Check struct {
	Type      string `yaml:"type"`
	Namespace string `yaml:"namespace,omitempty"`
	Name      string `yaml:"name,omitempty"`
	Condition string `yaml:"condition,omitempty"`
}

// Scope constrains what namespaces and resources the agent may touch.
type Scope struct {
	Namespaces []string `yaml:"namespaces,omitempty"`
	Deny       []string `yaml:"deny,omitempty"`
}

// BootstrapStep describes an environment preparation step.
type BootstrapStep struct {
	Name      string   `yaml:"name,omitempty"`
	Type      string   `yaml:"type"`
	Path      string   `yaml:"path,omitempty"`
	Release   string   `yaml:"release,omitempty"`
	Namespace string   `yaml:"namespace,omitempty"`
	Duration  string   `yaml:"duration,omitempty"`
	Args      []string `yaml:"args,omitempty"`
}

// Break describes how to inject a failure into the environment.
type Break struct {
	Type         string   `yaml:"type"`
	Path         string   `yaml:"path,omitempty"`
	Chart        string   `yaml:"chart,omitempty"`
	Name         string   `yaml:"name,omitempty"`
	Namespace    string   `yaml:"namespace,omitempty"`
	Command      string   `yaml:"command,omitempty"`
	Args         []string `yaml:"args,omitempty"`
	AllowFailure bool     `yaml:"allow_failure,omitempty"`
	Memory       string   `yaml:"memory,omitempty"` // "compact" | "reset", empty = full context
}

// Stage describes one phase of a multi-stage puzzle.
type Stage struct {
	Name       string          `yaml:"name"`
	Break      Break           `yaml:"break"`
	AfterBreak []BootstrapStep `yaml:"after_break,omitempty"`
	Checks     []Check         `yaml:"verify"`
	Trap       *Trap           `yaml:"trap,omitempty"`
	AgentGoal  string          `yaml:"agent_goal,omitempty"`
	OnFail     string          `yaml:"on_fail,omitempty"`
	Timeout    Duration        `yaml:"timeout,omitempty"`
}

// Trap describes bad agent behavior to detect.
type Trap struct {
	Name   string `yaml:"name"`
	Detect string `yaml:"detect"`
	Points int    `yaml:"points,omitempty"`
}

// ChaosConfig describes runtime disruptions scheduled during agent execution.
type ChaosConfig struct {
	Mode            string      `yaml:"mode,omitempty"`
	StopOnAgentDone bool        `yaml:"stop_on_agent_done,omitempty"`
	Steps           []ChaosStep `yaml:"steps,omitempty"`
}

// ChaosStep describes one scheduled runtime disruption.
type ChaosStep struct {
	Name         string   `yaml:"name,omitempty"`
	Type         string   `yaml:"type"`
	At           Duration `yaml:"at"`
	Path         string   `yaml:"path,omitempty"`
	Release      string   `yaml:"release,omitempty"`
	Namespace    string   `yaml:"namespace,omitempty"`
	Duration     string   `yaml:"duration,omitempty"`
	Args         []string `yaml:"args,omitempty"`
	AllowFailure bool     `yaml:"allow_failure,omitempty"`
}

// EvidraExpectations declares protocol compliance assertions for a scenario.
type EvidraExpectations struct {
	Enabled               bool           `yaml:"enabled"`
	MinPrescriptions      int            `yaml:"min_prescriptions,omitempty"`
	MinReports            int            `yaml:"min_reports,omitempty"`
	OrphanedPrescriptions int            `yaml:"orphaned_prescriptions,omitempty"`
	ProtocolViolations    int            `yaml:"protocol_violations,omitempty"`
	AllReportsHaveVerdict bool           `yaml:"all_reports_have_verdict,omitempty"`
	ExpectedRiskLevel     string         `yaml:"expected_risk_level,omitempty"`
	ExpectedRiskTags      []string       `yaml:"expected_risk_tags,omitempty"`
	DeclinedMin           int            `yaml:"declined_verdicts_min,omitempty"`
	DeclinedMax           *int           `yaml:"declined_verdicts_max,omitempty"`
	RetryLoopMax          int            `yaml:"retry_loop_max,omitempty"`
	ExpectedSignals       map[string]int `yaml:"expected_signals,omitempty"`
	SimulatedEvidenceDir  string         `yaml:"simulated_evidence_dir,omitempty"`
}

// Duration wraps time.Duration for YAML unmarshaling.
type Duration struct {
	time.Duration
	Set bool
}

// UnmarshalYAML parses a duration string like "5m" or "30s".
func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = dur
	d.Set = true
	return nil
}
