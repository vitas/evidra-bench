package evaluation

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type preflightFunc func(context.Context, Plan) error

func (f preflightFunc) Check(ctx context.Context, plan Plan) error { return f(ctx, plan) }

type provisionerFunc func(context.Context, Plan) (Lease, error)

func (f provisionerFunc) Acquire(ctx context.Context, plan Plan) (Lease, error) {
	return f(ctx, plan)
}

type executorFunc func(context.Context, Plan, CasePlan, Lease) (CaseResult, error)

func (f executorFunc) Execute(ctx context.Context, plan Plan, c CasePlan, lease Lease) (CaseResult, error) {
	return f(ctx, plan, c, lease)
}

type fakeLease struct {
	releases int
	ctxErr   error
	err      error
}

func (l *fakeLease) Release(ctx context.Context) error {
	l.releases++
	l.ctxErr = ctx.Err()
	return l.err
}

func validServicePlan() Plan {
	return Plan{
		Suite:       SuitePlan{ID: "demo@1", Digest: "sha256:abc", Cases: []CasePlan{{ID: "repair"}, {ID: "scope"}}},
		Environment: EnvironmentPlan{Provider: "kind", Profile: "default"},
		Target:      TargetPlan{Kind: TargetModel, Provider: "fake", Model: "fake-model"},
		Limits:      Limits{CaseTimeout: time.Minute},
		Attempts:    1,
	}
}

func TestServiceStopsBeforeProvisioningWhenPreflightFails(t *testing.T) {
	acquired := false
	service := Service{
		Preflights: []Preflight{preflightFunc(func(context.Context, Plan) error { return errors.New("docker unavailable") })},
		Provisioner: provisionerFunc(func(context.Context, Plan) (Lease, error) {
			acquired = true
			return &fakeLease{}, nil
		}),
	}

	result, err := service.Run(context.Background(), validServicePlan())
	if err == nil || err.Error() != "preflight: docker unavailable" {
		t.Fatalf("Run() error = %v", err)
	}
	if acquired {
		t.Fatal("provisioner called after failed preflight")
	}
	if result.Termination.Kind != TerminationIncomplete || result.Termination.Phase != "preflight" {
		t.Fatalf("termination = %+v", result.Termination)
	}
}

func TestServiceExecutesCasesInOrderAndReleasesOnce(t *testing.T) {
	lease := &fakeLease{}
	var executed []string
	service := Service{
		Provisioner: provisionerFunc(func(context.Context, Plan) (Lease, error) { return lease, nil }),
		Executor: executorFunc(func(_ context.Context, _ Plan, c CasePlan, _ Lease) (CaseResult, error) {
			executed = append(executed, c.ID)
			return CaseResult{ScenarioID: c.ID, Verdict: VerdictPass, Termination: Termination{Kind: TerminationComplete}}, nil
		}),
		ID:             func() string { return "eval-1" },
		Now:            func() time.Time { return time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC) },
		CleanupTimeout: time.Second,
	}

	result, err := service.Run(context.Background(), validServicePlan())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !reflect.DeepEqual(executed, []string{"repair", "scope"}) {
		t.Fatalf("executed = %v", executed)
	}
	if lease.releases != 1 || lease.ctxErr != nil {
		t.Fatalf("release calls/context = %d/%v", lease.releases, lease.ctxErr)
	}
	if result.ID != "eval-1" || result.Termination.Kind != TerminationComplete || result.Summary.Passed != 2 {
		t.Fatalf("result = %+v", result)
	}
	if !result.Cleanup.Attempted || !result.Cleanup.Succeeded {
		t.Fatalf("cleanup = %+v", result.Cleanup)
	}
}

func TestServiceReleasesWithDetachedContextAfterExecutionCancellation(t *testing.T) {
	lease := &fakeLease{}
	ctx, cancel := context.WithCancel(context.Background())
	service := Service{
		Provisioner: provisionerFunc(func(context.Context, Plan) (Lease, error) { return lease, nil }),
		Executor: executorFunc(func(_ context.Context, _ Plan, c CasePlan, _ Lease) (CaseResult, error) {
			cancel()
			return CaseResult{ScenarioID: c.ID, Verdict: VerdictIncomplete, Termination: Termination{Kind: TerminationCancelled}}, context.Canceled
		}),
		CleanupTimeout: time.Second,
	}

	result, err := service.Run(ctx, validServicePlan())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context canceled", err)
	}
	if lease.releases != 1 || lease.ctxErr != nil {
		t.Fatalf("release calls/context = %d/%v", lease.releases, lease.ctxErr)
	}
	if len(result.Cases) != 1 || result.Cases[0].Verdict != VerdictIncomplete {
		t.Fatalf("cases = %+v", result.Cases)
	}
}

func TestServiceMakesCleanupFailureVisible(t *testing.T) {
	lease := &fakeLease{err: errors.New("delete cluster")}
	service := Service{
		Provisioner: provisionerFunc(func(context.Context, Plan) (Lease, error) { return lease, nil }),
		Executor: executorFunc(func(_ context.Context, _ Plan, c CasePlan, _ Lease) (CaseResult, error) {
			return CaseResult{ScenarioID: c.ID, Verdict: VerdictPass, Termination: Termination{Kind: TerminationComplete}}, nil
		}),
		CleanupTimeout: time.Second,
	}

	result, err := service.Run(context.Background(), validServicePlan())
	if err == nil || err.Error() != "cleanup: delete cluster" {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Cleanup.Succeeded || result.Cleanup.Error != "delete cluster" || ExitCode(result) != 2 {
		t.Fatalf("cleanup/result = %+v, exit=%d", result.Cleanup, ExitCode(result))
	}
}
