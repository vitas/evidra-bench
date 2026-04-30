// Package timeline classifies agent tool calls into decision phases.
package bench

import "encoding/json"

// Phase represents a step in the agent's decision-making process.
type Phase string

const (
	PhaseDiscover Phase = "discover"
	PhaseDiagnose Phase = "diagnose"
	PhaseDecide   Phase = "decide"
	PhaseAct      Phase = "act"
	PhaseVerify   Phase = "verify"
	PhaseExplain  Phase = "explain"
)

// ToolCall matches the structure in tool-calls.json artifacts.
type ToolCall struct {
	Tool      string          `json:"tool"`
	Args      json.RawMessage `json:"args"`
	Result    string          `json:"result"`
	Timestamp string          `json:"timestamp"`
}

// TimelineStep is a single classified step in the decision timeline.
type TimelineStep struct {
	Index     int    `json:"index"`
	Phase     Phase  `json:"phase"`
	Tool      string `json:"tool"`
	Operation string `json:"operation"`
	Command   string `json:"command"`
	Resource  string `json:"resource,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	ExitCode  int    `json:"exit_code"`
	Summary   string `json:"summary"`
}

// Timeline is the full classified sequence of agent actions.
type Timeline struct {
	Steps         []TimelineStep `json:"steps"`
	PhaseCount    map[Phase]int  `json:"phase_count"`
	MutationCount int            `json:"mutation_count"`
	TotalSteps    int            `json:"total_steps"`
	// DiagnosisDepth counts diagnose-phase steps, which by construction
	// only occur before the first mutation.
	DiagnosisDepth int `json:"diagnosis_depth"`
}
