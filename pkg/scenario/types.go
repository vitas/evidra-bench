// Package scenario defines the scenario model and loader.
package scenario

import "time"

// Scenario is the parsed representation of a scenario.yaml file.
type Scenario struct {
	ID       string   `yaml:"id"`
	Title    string   `yaml:"title"`
	Category string   `yaml:"category"`
	Tags     []string `yaml:"tags,omitempty"`
	Prompt   string   `yaml:"prompt"`
	Timeout  Duration `yaml:"timeout,omitempty"`
	Checks   []Check  `yaml:"checks"`
	Scope    Scope    `yaml:"scope,omitempty"`
	Break    Break    `yaml:"break"`
	Baseline string   `yaml:"baseline,omitempty"`
	Tools    []string `yaml:"tools,omitempty"`
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

// Break describes how to inject a failure into the environment.
type Break struct {
	Type    string `yaml:"type"`
	Path    string `yaml:"path,omitempty"`
	Command string `yaml:"command,omitempty"`
}

// Duration wraps time.Duration for YAML unmarshaling.
type Duration struct {
	time.Duration
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
	return nil
}
