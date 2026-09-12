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

	// armed maps after_change steps to the resourceVersion observed
	// BEFORE the agent started (captured by ArmResourceTriggers).
	armed map[string]string
	// observed records the resourceVersion that fired the trigger.
	observed map[string]string
	// triggerErr is the first fatal trigger-watch fault (evaluator-side);
	// the harness maps it to INCOMPLETE — never a behavioral FAIL.
	triggerErr error
}

// TriggerFaulted reports a fatal evaluator-side chaos trigger failure.
func (r *ChaosRunner) TriggerFaulted() bool { return r != nil && r.triggerErr != nil }

// TriggerFault returns the fault to surface in the run error (nil-safe).
func (r *ChaosRunner) TriggerFault() error {
	if r == nil {
		return nil
	}
	return r.triggerErr
}

// ArmResourceTriggers synchronously captures the initial resourceVersion
// of every after_change step's watched object. A required trigger that
// cannot be armed MUST prevent the agent from starting (round: plan
// Task 3) — the caller maps the error to INCOMPLETE.
func (r *ChaosRunner) ArmResourceTriggers(ctx context.Context) error {
	for _, step := range r.Config.Steps {
		if step.AfterChange == nil {
			continue
		}
		rv, err := r.resourceVersion(ctx, step.AfterChange)
		if err != nil {
			return fmt.Errorf("chaos step %q: arm trigger: %w", step.Name, err)
		}
		if r.armed == nil {
			r.armed = map[string]string{}
		}
		r.armed[step.Name] = rv
	}
	return nil
}

// resourceVersion reads ONLY metadata.resourceVersion of the watched
// object (kubectl get --raw + targeted decode), so no object payload —
// let alone secret data — ever reaches chaos.json.
func (r *ChaosRunner) resourceVersion(ctx context.Context, ac *scenario.ResourceChangeTrigger) (string, error) {
	prefix := "/api/v1"
	if ac.APIVersion != "v1" && ac.APIVersion != "" {
		prefix = "/apis/" + strings.Trim(ac.APIVersion, "/")
	}
	path := fmt.Sprintf("%s/namespaces/%s/%s/%s", prefix, ac.Namespace, strings.ToLower(ac.Resource), ac.Name)
	//nolint:gosec // argv is fixed here; trigger fields are loader-validated.
	cmd := makeCmd([]string{"kubectl", "--kubeconfig", r.KubeconfigPath, "get", "--raw", path})
	out, err := r.Runner.Run(ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("read %s: %v", path, err)
	}
	var obj struct {
		Metadata struct {
			ResourceVersion string `json:"resourceVersion"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(out, &obj); err != nil {
		return "", fmt.Errorf("read %s: unparseable response: %v", path, err)
	}
	if obj.Metadata.ResourceVersion == "" {
		return "", fmt.Errorf("read %s: empty resourceVersion", path)
	}
	return obj.Metadata.ResourceVersion, nil
}

// waitForResourceChange polls (bounded 200ms) until the object's
// resourceVersion differs from the armed value. Returns the fire time.
// context.Canceled = agent finished first (step legitimately cancelled,
// not a fault); any other read error is a trigger fault.
func (r *ChaosRunner) waitForResourceChange(ctx context.Context, step scenario.ChaosStep, initial string) (time.Time, error) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return time.Time{}, ctx.Err()
		case <-ticker.C:
			current, err := r.resourceVersion(ctx, step.AfterChange)
			if err != nil {
				return time.Time{}, err
			}
			if current != initial {
				if r.observed == nil {
					r.observed = map[string]string{}
				}
				r.observed[step.Name] = current
				return time.Now(), nil
			}
		}
	}
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
			var (
				scheduledAt time.Time
				trigger     string
			)
			if step.AfterChange != nil {
				initial := r.armed[step.Name]
				if initial == "" {
					// Unarmed required trigger: never run the step blind.
					r.triggerErr = fmt.Errorf("chaos step %q: trigger was never armed", step.Name)
					r.events = append(r.events, chaosEvent{Name: step.Name, Type: step.Type, Trigger: "after_change", ScheduledAt: time.Now(), FinishedAt: time.Now(), Error: r.triggerErr.Error()})
					return
				}
				fireAt, err := r.waitForResourceChange(ctx, step, initial)
				if err != nil {
					if ctx.Err() != nil {
						return // agent finished first: step legitimately cancelled
					}
					r.triggerErr = fmt.Errorf("chaos step %q: %w", step.Name, err)
					r.events = append(r.events, chaosEvent{
						Name: step.Name, Type: step.Type, Trigger: "after_change",
						ScheduledAt: cycleStart.Add(step.At.Duration), FinishedAt: time.Now(),
						InitialResourceVersion: initial, Error: r.triggerErr.Error(),
					})
					return
				}
				scheduledAt, trigger = fireAt, "after_change"
			} else {
				scheduledAt = cycleStart.Add(step.At.Duration)
				trigger = "timer"
				if err := waitForChaosStep(ctx, cycleStart, step.At.Duration); err != nil {
					return
				}
			}
			event := r.executeStep(ctx, step, scheduledAt)
			event.Trigger = trigger
			if step.AfterChange != nil {
				event.InitialResourceVersion = r.armed[step.Name]
				event.ObservedResourceVersion = r.observed[step.Name]
			}
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
	Name string `json:"name"`
	Type string `json:"type"`
	// Trigger is "timer" or "after_change"; the versions describe an
	// after_change firing (payloads are never recorded).
	Trigger                 string    `json:"trigger,omitempty"`
	InitialResourceVersion  string    `json:"initial_resource_version,omitempty"`
	ObservedResourceVersion string    `json:"observed_resource_version,omitempty"`
	ScheduledAt             time.Time `json:"scheduled_at"`
	StartedAt               time.Time `json:"started_at"`
	FinishedAt              time.Time `json:"finished_at"`
	Command                 []string  `json:"command,omitempty"`
	Success                 bool      `json:"success"`
	AllowFailure            bool      `json:"allow_failure,omitempty"`
	Output                  string    `json:"output,omitempty"`
	Error                   string    `json:"error,omitempty"`
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
