package evaluation

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const defaultCleanupTimeout = 30 * time.Second

type Preflight interface {
	Check(context.Context, Plan) error
}

type Lease interface {
	Release(context.Context) error
}

type Provisioner interface {
	Acquire(context.Context, Plan) (Lease, error)
}

type CaseExecutor interface {
	Execute(context.Context, Plan, CasePlan, Lease) (CaseResult, error)
}

// Service owns the application-level evaluation lifecycle. Concrete adapters
// translate its small interfaces to the existing environment and harness APIs.
type Service struct {
	Preflights     []Preflight
	Provisioner    Provisioner
	Executor       CaseExecutor
	ID             func() string
	Now            func() time.Time
	CleanupTimeout time.Duration
}

func (s Service) Run(ctx context.Context, plan Plan) (result Result, runErr error) {
	return s.run(ctx, plan, nil, false)
}

// RunWithLease executes a plan on an environment owned by the caller. The
// caller remains responsible for releasing the lease after the evaluation.
func (s Service) RunWithLease(ctx context.Context, plan Plan, lease Lease) (result Result, runErr error) {
	return s.run(ctx, plan, lease, true)
}

func (s Service) run(ctx context.Context, plan Plan, lease Lease, borrowed bool) (result Result, runErr error) {
	now := s.Now
	if now == nil {
		now = time.Now
	}
	startedAt := now()
	result = Result{
		Version:     ResultVersion,
		Suite:       plan.Suite,
		Environment: plan.Environment,
		Target:      plan.Target,
		StartedAt:   startedAt,
		Termination: Termination{Kind: TerminationIncomplete, Phase: "validation"},
	}
	if s.ID != nil {
		result.ID = s.ID()
	} else {
		result.ID = fmt.Sprintf("evaluation-%d", startedAt.UTC().UnixNano())
	}

	if err := plan.Validate(); err != nil {
		result.Termination.Reason = "invalid_plan"
		result.Termination.Details = err.Error()
		result.EndedAt = now()
		return result, err
	}
	fingerprint, err := plan.Fingerprint()
	if err != nil {
		result.Termination.Reason = "fingerprint_failed"
		result.Termination.Details = err.Error()
		result.EndedAt = now()
		return result, err
	}
	result.PlanFingerprint = fingerprint

	result.Termination.Phase = "preflight"
	for _, check := range s.Preflights {
		if check == nil {
			continue
		}
		if err := check.Check(ctx, plan); err != nil {
			result.Termination.Reason = "preflight_failed"
			result.Termination.Details = err.Error()
			result.EndedAt = now()
			return result, fmt.Errorf("preflight: %w", err)
		}
	}

	if !borrowed && s.Provisioner == nil {
		err := errors.New("evaluation service: provisioner is required")
		result.Termination = Termination{Kind: TerminationIncomplete, Phase: "environment", Reason: "missing_provisioner", Details: err.Error()}
		result.EndedAt = now()
		return result, err
	}
	if s.Executor == nil {
		err := errors.New("evaluation service: case executor is required")
		result.Termination = Termination{Kind: TerminationIncomplete, Phase: "execution", Reason: "missing_executor", Details: err.Error()}
		result.EndedAt = now()
		return result, err
	}

	result.Termination.Phase = "environment"
	if !borrowed {
		lease, err = s.Provisioner.Acquire(ctx, plan)
		if err != nil {
			result.Termination.Reason = "acquire_failed"
			result.Termination.Details = err.Error()
			result.EndedAt = now()
			return result, fmt.Errorf("environment: %w", err)
		}
	}
	if lease == nil {
		err := errors.New("provisioner returned a nil lease")
		result.Termination.Reason = "invalid_lease"
		result.Termination.Details = err.Error()
		result.EndedAt = now()
		return result, fmt.Errorf("environment: %w", err)
	}

	if !borrowed {
		defer func() {
			cleanup, cleanupErr := ReleaseLease(ctx, lease, s.CleanupTimeout)
			result.Cleanup = cleanup
			if cleanupErr != nil {
				if runErr == nil {
					runErr = cleanupErr
				} else {
					runErr = errors.Join(runErr, cleanupErr)
				}
			}
			result.Summary = Summarize(result)
			result.EndedAt = now()
		}()
	}

	result.Termination.Phase = "execution"
	for _, c := range plan.Suite.Cases {
		caseResult, err := s.Executor.Execute(ctx, plan, c, lease)
		if caseResult.ScenarioID == "" {
			caseResult.ScenarioID = c.ID
		}
		if err != nil {
			if caseResult.Verdict == "" {
				caseResult.Verdict = VerdictIncomplete
			}
			if caseResult.Termination.Kind == "" {
				caseResult.Termination = Termination{Kind: TerminationIncomplete, Phase: "execution", Reason: "case_execution_failed", Details: err.Error()}
			}
			result.Cases = append(result.Cases, caseResult)
			result.Termination = Termination{Kind: TerminationIncomplete, Phase: "execution", Reason: "case_execution_failed", Details: err.Error()}
			if errors.Is(err, context.Canceled) {
				result.Termination.Kind = TerminationCancelled
			}
			result.Summary = Summarize(result)
			result.EndedAt = now()
			return result, err
		}
		result.Cases = append(result.Cases, caseResult)
	}

	result.Termination = Termination{Kind: TerminationComplete}
	result.Summary = Summarize(result)
	result.EndedAt = now()
	return result, nil
}
