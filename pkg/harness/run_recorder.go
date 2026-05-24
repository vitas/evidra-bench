package harness

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

const runErrorVersion = "run-error.v1"

type runEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Phase     string    `json:"phase"`
	Status    string    `json:"status"`
	Message   string    `json:"message,omitempty"`
}

type runErrorArtifact struct {
	Version   string    `json:"version"`
	Phase     string    `json:"phase"`
	Kind      string    `json:"kind"`
	Message   string    `json:"message"`
	Retryable bool      `json:"retryable"`
	ExitCode  int       `json:"exit_code"`
	Timestamp time.Time `json:"timestamp"`
}

type runArtifactRecorder struct {
	events []runEvent
	phase  string
}

func newRunArtifactRecorder(started time.Time) *runArtifactRecorder {
	r := &runArtifactRecorder{}
	r.addAt(started, "run", "started", "")
	return r
}

func (r *runArtifactRecorder) Event(phase, status, message string) {
	r.addAt(time.Now(), phase, status, message)
}

func (r *runArtifactRecorder) addAt(ts time.Time, phase, status, message string) {
	if r == nil {
		return
	}
	if phase != "" {
		r.phase = phase
	}
	r.events = append(r.events, runEvent{
		Timestamp: ts,
		Phase:     phase,
		Status:    status,
		Message:   message,
	})
}

func (r *runArtifactRecorder) CurrentPhase() string {
	if r == nil || r.phase == "" {
		return "run"
	}
	return r.phase
}

func (r *runArtifactRecorder) EventsJSON() json.RawMessage {
	if r == nil {
		return nil
	}
	data, err := json.MarshalIndent(r.events, "", "  ")
	if err != nil {
		return nil
	}
	return data
}

func buildRunErrorArtifact(err error, phase string, exitCode int, ts time.Time) runErrorArtifact {
	kind, retryable := classifyRunError(err, phase)
	return runErrorArtifact{
		Version:   runErrorVersion,
		Phase:     phase,
		Kind:      kind,
		Message:   err.Error(),
		Retryable: retryable,
		ExitCode:  exitCode,
		Timestamp: ts,
	}
}

func buildRunErrorJSON(runErr runErrorArtifact) json.RawMessage {
	data, err := json.MarshalIndent(runErr, "", "  ")
	if err != nil {
		return nil
	}
	return data
}

func classifyRunError(err error, phase string) (kind string, retryable bool) {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout", true
	}
	if errors.Is(err, context.Canceled) {
		return "context_canceled", true
	}
	var infraErr *InfraError
	if errors.As(err, &infraErr) {
		return "infra_error", true
	}

	switch phase {
	case "configuration":
		return "configuration_error", false
	case "environment", "bootstrap", "break":
		return "setup_error", true
	case "agent_prepare":
		return "configuration_error", false
	case "agent_run":
		return "adapter_error", false
	case "verification":
		return "verifier_error", false
	default:
		return "run_error", false
	}
}

func failedRunExitCode(err error, agentResultExitCode int) int {
	if agentResultExitCode != 0 {
		return agentResultExitCode
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return -1
	}
	var infraErr *InfraError
	if errors.As(err, &infraErr) {
		return -1
	}
	return -1
}
