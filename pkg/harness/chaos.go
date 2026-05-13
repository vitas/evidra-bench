package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/vitas/evidra-bench/pkg/environment"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

// ChaosRunner executes runtime disruption steps while an agent is running.
type ChaosRunner struct {
	Runner         environment.CommandRunner
	KubeconfigPath string
	Config         scenario.ChaosConfig
	events         []chaosEvent
}

// Run executes the configured chaos schedule until completion or cancellation.
func (r *ChaosRunner) Run(ctx context.Context) {
	mode := r.Config.Mode
	if mode == "" {
		mode = "once"
	}
	for {
		cycleStart := time.Now()
		for _, step := range r.Config.Steps {
			scheduledAt := cycleStart.Add(step.At.Duration)
			if err := waitForChaosStep(ctx, cycleStart, step.At.Duration); err != nil {
				return
			}
			event := r.executeStep(ctx, step, scheduledAt)
			r.events = append(r.events, event)
			if event.Error != "" {
				if step.AllowFailure {
					log.Printf("[chaos] step %s failed as allowed: %s", step.Name, event.Error)
					continue
				}
				log.Printf("[chaos] step %s failed: %s", step.Name, event.Error)
			}
		}
		if mode != "repeat" {
			return
		}
	}
}

func (r *ChaosRunner) executeStep(ctx context.Context, step scenario.ChaosStep, scheduledAt time.Time) chaosEvent {
	event := chaosEvent{
		Name:         step.Name,
		Type:         step.Type,
		ScheduledAt:  scheduledAt,
		AllowFailure: step.AllowFailure,
		Command:      chaosCommandArgs(r.KubeconfigPath, step),
	}
	event.StartedAt = time.Now()
	if step.Type == "sleep" {
		duration, err := time.ParseDuration(step.Duration)
		if err != nil {
			event.FinishedAt = time.Now()
			event.Error = fmt.Sprintf("parse chaos sleep duration %q: %v", step.Duration, err)
			return event
		}
		if err := sleepContext(ctx, duration); err != nil {
			event.FinishedAt = time.Now()
			event.Error = err.Error()
			return event
		}
		event.FinishedAt = time.Now()
		event.Success = true
		return event
	}

	envStep := environment.BootstrapStep{
		Name:      step.Name,
		Type:      environment.StepType(step.Type),
		Path:      step.Path,
		Release:   step.Release,
		Namespace: step.Namespace,
		Duration:  step.Duration,
		Args:      append([]string(nil), step.Args...),
	}
	args := envStep.CommandArgs(r.KubeconfigPath)
	if len(args) == 0 {
		event.FinishedAt = time.Now()
		event.Error = fmt.Sprintf("no command for chaos step %s", step.Name)
		return event
	}
	cmd := makeCmd(args)
	out, err := r.Runner.Run(ctx, cmd)
	event.FinishedAt = time.Now()
	event.Output = strings.TrimSpace(string(out))
	if err != nil {
		event.Error = err.Error()
		return event
	}
	event.Success = true
	return event
}

func waitForChaosStep(ctx context.Context, cycleStart time.Time, at time.Duration) error {
	wait := at - time.Since(cycleStart)
	if wait <= 0 {
		return nil
	}
	return sleepContext(ctx, wait)
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func shouldCancelChaosOnAgentDone(cfg scenario.ChaosConfig) bool {
	return cfg.StopOnAgentDone || cfg.Mode == "repeat"
}

func chaosCommandArgs(kubeconfigPath string, step scenario.ChaosStep) []string {
	if step.Type == "sleep" {
		return nil
	}
	envStep := environment.BootstrapStep{
		Name:      step.Name,
		Type:      environment.StepType(step.Type),
		Path:      step.Path,
		Release:   step.Release,
		Namespace: step.Namespace,
		Duration:  step.Duration,
		Args:      append([]string(nil), step.Args...),
	}
	return envStep.CommandArgs(kubeconfigPath)
}

func chaosArtifacts(r *ChaosRunner) (json.RawMessage, string) {
	if r == nil {
		return nil, ""
	}
	summary := r.Snapshot()
	if len(summary.Events) == 0 {
		return nil, ""
	}
	data, err := json.Marshal(summary)
	if err != nil {
		return nil, ""
	}
	return data, summary.Log()
}

// Snapshot returns the executed chaos timeline.
func (r *ChaosRunner) Snapshot() chaosSummary {
	mode := r.Config.Mode
	if mode == "" {
		mode = "once"
	}
	events := append([]chaosEvent(nil), r.events...)
	return chaosSummary{
		Mode:            mode,
		StopOnAgentDone: shouldCancelChaosOnAgentDone(r.Config),
		Events:          events,
	}
}

type chaosSummary struct {
	Mode            string       `json:"mode"`
	StopOnAgentDone bool         `json:"stop_on_agent_done"`
	Events          []chaosEvent `json:"events"`
}

type chaosEvent struct {
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	ScheduledAt  time.Time `json:"scheduled_at"`
	StartedAt    time.Time `json:"started_at"`
	FinishedAt   time.Time `json:"finished_at"`
	Command      []string  `json:"command,omitempty"`
	Success      bool      `json:"success"`
	AllowFailure bool      `json:"allow_failure,omitempty"`
	Output       string    `json:"output,omitempty"`
	Error        string    `json:"error,omitempty"`
}

func (s chaosSummary) Log() string {
	var b strings.Builder
	for _, event := range s.Events {
		status := "ok"
		if event.Error != "" {
			status = "error: " + event.Error
		}
		fmt.Fprintf(&b,
			"%s %s %s %s\n",
			event.StartedAt.Format(time.RFC3339Nano),
			event.Name,
			event.Type,
			status,
		)
	}
	return b.String()
}
