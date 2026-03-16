// Package executor manages async benchmark run execution.
package executor

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"samebits.com/evidra-infra-bench/pkg/adapter"
	"samebits.com/evidra-infra-bench/pkg/artifact"
	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/environment"
	"samebits.com/evidra-infra-bench/pkg/harness"
	"samebits.com/evidra-infra-bench/pkg/report"
	"samebits.com/evidra-infra-bench/pkg/scenario"
	"samebits.com/evidra-infra-bench/pkg/store"
)

// ExecuteRequest describes a run to execute.
type ExecuteRequest struct {
	ScenarioID string `json:"scenario_id"`
	Model      string `json:"model"`
	Provider   string `json:"provider"`
	DryRun     bool   `json:"dry_run"`
}

// Job tracks an in-progress or completed execution.
type Job struct {
	ID         string     `json:"id"`
	ScenarioID string     `json:"scenario_id"`
	Model      string     `json:"model"`
	Provider   string     `json:"provider"`
	Status     string     `json:"status"` // pending, running, completed, failed
	StartedAt  time.Time  `json:"started_at"`
	EndedAt    *time.Time `json:"ended_at,omitempty"`
	RunID      *string    `json:"run_id,omitempty"`
	ExitCode   *int       `json:"exit_code,omitempty"`
	Passed     *bool      `json:"passed,omitempty"`
	Error      string     `json:"error,omitempty"`
}

// Executor manages benchmark run jobs.
type Executor struct {
	mu           sync.Mutex
	jobs         map[string]*Job
	running      map[string]bool // scenarioID -> running, prevents duplicates
	scenariosDir string
	runsDir      string
	store        *store.Store
}

// New creates an Executor.
func New(s *store.Store, scenariosDir, runsDir string) *Executor {
	return &Executor{
		jobs:         make(map[string]*Job),
		running:      make(map[string]bool),
		scenariosDir: scenariosDir,
		runsDir:      runsDir,
		store:        s,
	}
}

// Start begins a new scenario run in the background.
func (e *Executor) Start(ctx context.Context, req ExecuteRequest) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running[req.ScenarioID] {
		return "", fmt.Errorf("scenario %s is already running", req.ScenarioID)
	}

	jobID := fmt.Sprintf("job-%s-%d", req.ScenarioID, time.Now().UnixMilli())
	now := time.Now()

	job := &Job{
		ID:         jobID,
		ScenarioID: req.ScenarioID,
		Model:      req.Model,
		Provider:   req.Provider,
		Status:     "pending",
		StartedAt:  now,
	}

	e.jobs[jobID] = job
	e.running[req.ScenarioID] = true

	go e.run(job, req)

	return jobID, nil
}

// Status returns the current state of a job.
func (e *Executor) Status(jobID string) (*Job, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	job, ok := e.jobs[jobID]
	if !ok {
		return nil, fmt.Errorf("job %s not found", jobID)
	}
	// Return a copy to avoid data races.
	cp := *job
	return &cp, nil
}

func (e *Executor) run(job *Job, req ExecuteRequest) {
	defer func() {
		e.mu.Lock()
		delete(e.running, req.ScenarioID)
		e.mu.Unlock()
	}()

	e.mu.Lock()
	job.Status = "running"
	e.mu.Unlock()

	// Resolve scenario.
	s, err := scenario.Resolve(e.scenariosDir, req.ScenarioID)
	if err != nil {
		e.failJob(job, fmt.Errorf("resolve scenario: %w", err))
		return
	}

	// Build config.
	cfg := config.Default()
	cfg.Scenario = s.Path
	cfg.ScenariosDir = e.scenariosDir
	cfg.RunsDir = e.runsDir
	cfg.Model = req.Model
	cfg.Provider = req.Provider
	cfg.DryRun = req.DryRun
	cfg.ReuseCluster = true // API assumes cluster exists

	// Build harness deps.
	agentAdapter := adapter.NewCLIAdapter()

	envProvider := environment.NewKindProvider()
	envProvider.ReuseExisting = true
	runner := &environment.ExecRunner{}
	bootstrapper := environment.NewBootstrapper(runner)
	writer := artifact.NewWriter(cfg.RunsDir)

	reporter := report.NewReporter(report.Config{
		EvidencePath: filepath.Join(cfg.RunsDir, "evidra"),
	})

	deps := harness.Deps{
		EnvProvider:  envProvider,
		Bootstrapper: bootstrapper,
		Runner:       runner,
		Adapter:      agentAdapter,
		Writer:       writer,
		Reporter:     reporter,
		Store:        e.store,
	}

	h := harness.New(deps)

	result, err := h.Run(context.Background(), harness.RunRequest{
		Config:   cfg,
		Scenario: s,
	})
	if err != nil {
		e.failJob(job, err)
		return
	}

	e.mu.Lock()
	now := time.Now()
	job.Status = "completed"
	job.EndedAt = &now
	job.ExitCode = &result.ExitCode
	job.Passed = &result.Passed

	runID := fmt.Sprintf("%s-%s-%s", job.StartedAt.Format("20060102-150405"), s.ID, cfg.Adapter)
	job.RunID = &runID
	e.mu.Unlock()

	log.Printf("[executor] job %s completed: passed=%v", job.ID, result.Passed)
}

func (e *Executor) failJob(job *Job, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	job.Status = "failed"
	job.EndedAt = &now
	job.Error = err.Error()
	log.Printf("[executor] job %s failed: %v", job.ID, err)
}
