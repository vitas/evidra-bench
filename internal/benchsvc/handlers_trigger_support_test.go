package benchsvc

import (
	"context"
)

// spyExecutor records whether Start was called.
type spyExecutor struct {
	started   bool
	job       *TriggerJob
	ctxValue  any
	startedCh chan struct{}
}

func (e *spyExecutor) Start(ctx context.Context, job *TriggerJob, _, _ string) error {
	e.started = true
	e.job = job
	e.ctxValue = ctx.Value(testContextKey{})
	if e.startedCh != nil {
		close(e.startedCh)
		e.startedCh = nil
	}
	return nil
}

type testContextKey struct{}
