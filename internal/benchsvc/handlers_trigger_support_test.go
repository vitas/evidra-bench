package benchsvc

import (
	"context"
)

// spyExecutor records whether Start was called.
type spyExecutor struct {
	started   bool
	job       *TriggerJob
	startedCh chan struct{}
}

func (e *spyExecutor) Start(_ context.Context, job *TriggerJob, _, _ string) error {
	e.started = true
	e.job = job
	if e.startedCh != nil {
		close(e.startedCh)
		e.startedCh = nil
	}
	return nil
}
