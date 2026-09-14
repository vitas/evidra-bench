package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vitas/evidra-bench/pkg/scenario"
)

// triggerRunner scripts kubectl get --raw responses and records applied
// chaos steps. Synchronization is via channels — no sleeps as assertions.
type triggerRunner struct {
	mu        sync.Mutex
	rv        string // current scripted resourceVersion
	failGet   bool
	failApply bool

	gets    chan struct{} // signaled on every raw read
	applied chan string   // signaled when the chaos step executes
}

func newTriggerRunner(initial string) *triggerRunner {
	return &triggerRunner{
		rv:      initial,
		gets:    make(chan struct{}, 64),
		applied: make(chan string, 16),
	}
}

func (t *triggerRunner) setRV(rv string) {
	t.mu.Lock()
	t.rv = rv
	t.mu.Unlock()
}

func (t *triggerRunner) setFailGet(f bool) {
	t.mu.Lock()
	t.failGet = f
	t.mu.Unlock()
}

func (t *triggerRunner) setFailApply(f bool) {
	t.mu.Lock()
	t.failApply = f
	t.mu.Unlock()
}

func (t *triggerRunner) Run(_ context.Context, cmd *exec.Cmd) ([]byte, error) {
	joined := strings.Join(cmd.Args, " ")
	if strings.Contains(joined, "get --raw") {
		t.mu.Lock()
		fail := t.failGet
		rv := t.rv
		t.mu.Unlock()
		if fail {
			return nil, fmt.Errorf("connection refused")
		}
		t.gets <- struct{}{}
		out, _ := json.Marshal(map[string]any{"metadata": map[string]string{"resourceVersion": rv}})
		return out, nil
	}
	t.applied <- joined
	t.mu.Lock()
	fail := t.failApply
	t.mu.Unlock()
	if fail {
		return nil, fmt.Errorf("error: server could not find requested resource")
	}
	return []byte("configured"), nil
}

func triggerScenario(steps ...scenario.ChaosStep) *scenario.Scenario {
	return &scenario.Scenario{ID: "config-mutation", Chaos: scenario.ChaosConfig{StopOnAgentDone: true, Steps: steps}}
}

func changeStep() scenario.ChaosStep {
	return scenario.ChaosStep{
		Name: "second-drift",
		Type: "kubectl-apply",
		Path: "fixtures/chaos-bad.yaml",
		AfterChange: &scenario.ResourceChangeTrigger{
			APIVersion: "v1", Resource: "configmaps", Namespace: "bench", Name: "web-config",
		},
	}
}

func waitGets(t *testing.T, r *triggerRunner, n int, what string) {
	t.Helper()
	for i := 0; i < n; i++ {
		select {
		case <-r.gets:
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for raw read %d (%s)", i+1, what)
		}
	}
}

func TestChaosTriggerFiresOnlyAfterObservedChange(t *testing.T) {
	r := newTriggerRunner("100")
	cr := &ChaosRunner{Runner: r, KubeconfigPath: "/kc", Config: triggerScenario(changeStep()).Chaos}

	// Pre-arm: initial resourceVersion captured synchronously, and NOTHING
	// has been applied yet.
	if err := cr.ArmResourceTriggers(context.Background()); err != nil {
		t.Fatal(err)
	}
	if cr.armed["second-drift"] != "100" {
		t.Fatalf("armed = %v", cr.armed)
	}
	waitGets(t, r, 1, "arm read")
	select {
	case got := <-r.applied:
		t.Fatalf("step fired before any change: %s", got)
	default:
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		cr.Run(ctx)
	}()

	// Poll while unchanged: no fire.
	waitGets(t, r, 2, "steady polls")
	select {
	case got := <-r.applied:
		t.Fatalf("step fired without a change: %s", got)
	case <-time.After(500 * time.Millisecond):
	}

	// The (simulated) agent write changes the object — now it must fire.
	r.setRV("101")
	select {
	case got := <-r.applied:
		if !strings.Contains(got, "apply -f fixtures/chaos-bad.yaml") {
			t.Fatalf("unexpected apply argv: %s", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("step never fired after resourceVersion changed")
	}
	cancel()
	<-done
	if cr.TriggerFaulted() {
		t.Fatalf("clean firing must not fault: %v", cr.TriggerFault())
	}
	var fired *chaosEvent
	for i := range cr.events {
		if cr.events[i].Trigger == "after_change" && cr.events[i].Success {
			fired = &cr.events[i]
		}
	}
	if fired == nil {
		t.Fatalf("no after_change event: %+v", cr.events)
	}
	if fired.InitialResourceVersion != "100" || fired.ObservedResourceVersion != "101" {
		t.Fatalf("versions not recorded: %+v", fired)
	}
}

func TestChaosTriggerCancelledByAgentCompletion(t *testing.T) {
	r := newTriggerRunner("100")
	cr := &ChaosRunner{Runner: r, KubeconfigPath: "/kc", Config: triggerScenario(changeStep()).Chaos}
	if err := cr.ArmResourceTriggers(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		cr.Run(ctx)
	}()
	waitGets(t, r, 2, "polls before agent done")
	cancel() // the agent finished before any resource change
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("runner did not stop on cancel")
	}
	select {
	case got := <-r.applied:
		t.Fatalf("cancelled step must never execute: %s", got)
	default:
	}
	if cr.TriggerFaulted() {
		t.Fatalf("agent-completion cancellation is not a fault: %v", cr.TriggerFault())
	}
}

func TestChaosTriggerWatchFailureIsEvaluatorFault(t *testing.T) {
	r := newTriggerRunner("100")
	cr := &ChaosRunner{Runner: r, KubeconfigPath: "/kc", Config: triggerScenario(changeStep()).Chaos}
	if err := cr.ArmResourceTriggers(context.Background()); err != nil {
		t.Fatal(err)
	}
	r.setFailGet(true)
	cr.Run(context.Background()) // watch fails on the first poll
	if !cr.TriggerFaulted() {
		t.Fatal("watch failure must mark a trigger fault (=> INCOMPLETE upstream)")
	}
	if len(cr.events) == 0 || !strings.Contains(cr.events[len(cr.events)-1].Error, "connection refused") {
		t.Fatalf("fault not recorded in events: %+v", cr.events)
	}
	if cr.events[len(cr.events)-1].InitialResourceVersion != "100" {
		t.Fatal("failed watch must still record the armed version")
	}
}

func TestChaosTriggerArmFailurePreventsStart(t *testing.T) {
	r := newTriggerRunner("")
	r.setFailGet(true)
	cr := &ChaosRunner{Runner: r, KubeconfigPath: "/kc", Config: triggerScenario(changeStep()).Chaos}
	if err := cr.ArmResourceTriggers(context.Background()); err == nil {
		t.Fatal("unarmable trigger must error before the agent starts")
	}
}

func TestChaosTimerStepsUnchanged(t *testing.T) {
	// A pure timer schedule behaves exactly as before (no trigger reads).
	r := newTriggerRunner("100")
	step := scenario.ChaosStep{Name: "poke", Type: "kubectl", Args: []string{"delete", "pod", "web"}, At: scenario.Duration{Duration: 0, Set: true}}
	cr := &ChaosRunner{Runner: r, KubeconfigPath: "/kc", Config: triggerScenario(step).Chaos}
	if err := cr.ArmResourceTriggers(context.Background()); err != nil {
		t.Fatal(err)
	}
	cr.Run(context.Background())
	select {
	case got := <-r.applied:
		if !strings.Contains(got, "delete pod web") {
			t.Fatalf("timer step argv changed shape: %s", got)
		}
	default:
		t.Fatal("timer step did not run")
	}
	for _, e := range cr.events {
		if e.Trigger != "timer" {
			t.Fatalf("timer event mislabeled: %+v", e)
		}
	}
	if len(r.gets) != 0 {
		t.Fatal("timer-only config must not read triggers")
	}
}

// A trigger-armed step is load-bearing for the case premise: its APPLY
// failure must fault the run exactly like a watch failure — a drift that
// silently never materialized means the evaluation proved nothing.
func TestChaosTriggerApplyFailureIsEvaluatorFault(t *testing.T) {
	r := newTriggerRunner("100")
	r.setFailApply(true)
	cr := &ChaosRunner{Runner: r, KubeconfigPath: "/kc", Config: triggerScenario(changeStep()).Chaos}
	if err := cr.ArmResourceTriggers(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		cr.Run(ctx)
	}()
	r.setRV("200") // the agent's change
	select {
	case <-r.applied:
	case <-time.After(5 * time.Second):
		t.Fatal("step never attempted")
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("runner did not stop after failed load-bearing step")
	}
	if !cr.TriggerFaulted() {
		t.Fatalf("apply failure must fault the trigger channel: triggerErr=%v", cr.triggerErr)
	}
}

// Timer steps keep their historical log-only failure behavior (pinned by
// TestChaosTimerStepsUnchanged; assert the trigger channel stays clean).
func TestChaosTimerFailureDoesNotFaultTrigger(t *testing.T) {
	r := newTriggerRunner("100")
	r.setFailApply(true)
	step := scenario.ChaosStep{Name: "t1", Type: "kubectl-apply", Path: "p.yaml", At: scenario.Duration{Duration: 10 * time.Millisecond}}
	cr := &ChaosRunner{Runner: r, KubeconfigPath: "/kc", Config: triggerScenario(step).Chaos}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cr.Run(ctx)
	if cr.TriggerFaulted() {
		t.Fatal("timer-step failure must not enter the trigger fault channel")
	}
	if len(cr.events) != 1 || cr.events[0].Success {
		t.Fatalf("events = %+v", cr.events)
	}
}
