// Package orchestrator manages the full bench lifecycle:
// provision cluster → run scenarios in parallel → teardown.
package orchestrator

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/environment"
	"samebits.com/evidra-infra-bench/pkg/jobqueue"
	"samebits.com/evidra-infra-bench/pkg/store"
	"samebits.com/evidra-infra-bench/pkg/workspace"
)

// RunFunc executes a single scenario. Called by the orchestrator for each job.
// cfg has workspace-scoped paths. s is loaded from the workspace copy.
// kubeconfigPath points to the pre-provisioned cluster.
// targetNS is the worker's isolated namespace.
// sharedStore persists results beyond workspace cleanup.
type RunFunc func(ctx context.Context, cfg config.Config, scenarioPath, targetNS, kubeconfigPath string, sharedStore *store.Store) error

// ScenarioEvent describes a scenario lifecycle transition for external reporting.
type ScenarioEvent struct {
	JobID      string        // Evidra trigger job ID
	ScenarioID string        // Scenario identifier
	Model      string        // LLM model name
	Provider   string        // LLM provider
	Status     string        // "running", "passed", "failed"
	RunID      string        // Unique run identifier (set on completion)
	Completed  int           // Scenarios completed so far
	Total      int           // Total scenarios in the job
	Duration   time.Duration // Scenario wall-clock duration (set on completion)
	Passed     bool          // Whether verification passed
}

// ProgressReporter receives scenario lifecycle events. Implementations must be
// safe for concurrent use when parallel > 1.
type ProgressReporter interface {
	OnScenario(ctx context.Context, event ScenarioEvent)
}

// Orchestrator coordinates provisioning, parallel execution, and teardown.
type Orchestrator struct {
	cfg         config.Config
	runFn       RunFunc
	cluster     *environment.Handle
	provider    environment.Provider
	ownsCluster bool
}

// New creates an orchestrator with the given config and scenario run function.
func New(cfg config.Config, runFn RunFunc) *Orchestrator {
	return &Orchestrator{cfg: cfg, runFn: runFn}
}

// Provision creates or reuses the target cluster. Must be called before Run.
// If KubeconfigPath is already set (e.g. via KUBECONFIG env), skips cluster
// provisioning and uses the provided kubeconfig directly.
func (o *Orchestrator) Provision(ctx context.Context) (string, error) {
	if o.cfg.KubeconfigPath != "" {
		o.cluster = &environment.Handle{
			ClusterName:    o.cfg.ClusterName,
			KubeconfigPath: o.cfg.KubeconfigPath,
		}
		o.ownsCluster = false
		log.Printf("[orchestrator] using existing kubeconfig: %s", o.cfg.KubeconfigPath)
		return o.cfg.KubeconfigPath, nil
	}

	switch o.cfg.EnvironmentProvider {
	case "k3d":
		p := environment.NewK3dProvider()
		p.ReuseExisting = o.cfg.ReuseCluster
		o.provider = p
	default:
		p := environment.NewKindProvider()
		p.ReuseExisting = o.cfg.ReuseCluster
		o.provider = p
	}

	handle, err := o.provider.Create(ctx, o.cfg.ClusterName)
	if err != nil {
		return "", fmt.Errorf("orchestrator: provision: %w", err)
	}
	o.cluster = handle
	o.ownsCluster = true
	log.Printf("[orchestrator] cluster %s ready, kubeconfig: %s", o.cfg.ClusterName, handle.KubeconfigPath)
	return handle.KubeconfigPath, nil
}

// Teardown destroys the cluster unless --reuse-cluster is set.
func (o *Orchestrator) Teardown(ctx context.Context) {
	if o.cluster == nil || o.cfg.ReuseCluster || !o.ownsCluster || o.provider == nil {
		return
	}
	log.Printf("[orchestrator] destroying cluster %s", o.cfg.ClusterName)
	if err := o.provider.Destroy(ctx, o.cluster); err != nil {
		log.Printf("[orchestrator] warning: destroy failed: %v", err)
	}
}

// KubeconfigPath returns the path to the provisioned cluster kubeconfig.
func (o *Orchestrator) KubeconfigPath() string {
	if o.cluster == nil {
		return ""
	}
	return o.cluster.KubeconfigPath
}

// RunResult holds aggregate results from a parallel run.
type RunResult struct {
	Total     int64
	Completed int64
	Passed    int64
	Failed    int64
}

// RunParallel enqueues and executes scenarios via River.
// Returns after all jobs complete or ctx is cancelled.
func (o *Orchestrator) RunParallel(ctx context.Context, runCfg config.Config, reporter ProgressReporter, scenarios []string, models []string, repeats, parallel int, dbURL string) (*RunResult, error) {
	if o.cluster == nil {
		return nil, fmt.Errorf("orchestrator: cluster not provisioned — call Provision first")
	}
	kubeconfigPath := o.cluster.KubeconfigPath

	// Open shared results store.
	sharedStore, err := store.Open(runCfg.RunsDir)
	if err != nil {
		log.Printf("[orchestrator] warning: could not open shared store: %v", err)
	}
	if sharedStore != nil {
		defer func() { _ = sharedStore.Close() }()
	}

	var completed, passed, failed int64
	total := int64(len(scenarios) * len(models) * repeats)
	cfg := runCfg
	runFn := o.runFn

	// Build River worker function.
	workerFn := func(jobCtx context.Context, args jobqueue.BenchJobArgs, ns string) error {
		ws, wsErr := workspace.New(
			fmt.Sprintf("%s-%s-%d", args.ScenarioID, args.Model, time.Now().UnixNano()),
			cfg.ScenariosDir,
		)
		if wsErr != nil {
			return fmt.Errorf("workspace: %w", wsErr)
		}
		defer ws.Cleanup()

		// Rewrite namespace across the entire workspace.
		if rwErr := workspace.RewriteNamespace(ws.Root, config.DefaultNamespace, ns); rwErr != nil {
			log.Printf("[worker-%d] namespace rewrite warning: %v", args.NamespaceSlot, rwErr)
		}

		workerCfg := cfg
		workerCfg.Scenario = args.ScenarioID
		workerCfg.ScenariosDir = ws.ScenariosDir
		workerCfg.RunsDir = ws.RunsDir
		workerCfg.EvidraEvidenceDir = ws.EvidenceDir
		workerCfg.Model = args.Model

		// Report scenario started.
		c := int(atomic.LoadInt64(&completed))
		if reporter != nil {
			reporter.OnScenario(jobCtx, ScenarioEvent{
				JobID:      args.JobID,
				ScenarioID: args.ScenarioID,
				Model:      args.Model,
				Provider:   args.Provider,
				Status:     "running",
				Completed:  c,
				Total:      int(total),
			})
		}

		scenarioPath := args.ScenarioID
		startTime := time.Now()
		runErr := runFn(jobCtx, workerCfg, scenarioPath, ns, kubeconfigPath, sharedStore)
		duration := time.Since(startTime)
		atomic.AddInt64(&completed, 1)
		c = int(atomic.LoadInt64(&completed))

		status := "passed"
		if runErr != nil {
			status = "failed"
			atomic.AddInt64(&failed, 1)
			log.Printf("[worker-%d] FAIL %s: %v", args.NamespaceSlot, args.ScenarioID, runErr)
		} else {
			atomic.AddInt64(&passed, 1)
			log.Printf("[worker-%d] PASS %s", args.NamespaceSlot, args.ScenarioID)
		}

		// Report scenario completed.
		if reporter != nil {
			runID := fmt.Sprintf("%s-%s-%s",
				time.Now().UTC().Format("20060102-150405"),
				args.ScenarioID, args.Model)
			reporter.OnScenario(jobCtx, ScenarioEvent{
				JobID:      args.JobID,
				ScenarioID: args.ScenarioID,
				Model:      args.Model,
				Provider:   args.Provider,
				Status:     status,
				RunID:      runID,
				Completed:  c,
				Total:      int(total),
				Duration:   duration,
				Passed:     runErr == nil,
			})
		}

		return nil // don't fail River job on scenario failure
	}

	// Create River client.
	client, err := jobqueue.NewClient(ctx, dbURL, parallel, workerFn)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: job queue: %w", err)
	}
	if err := client.Migrate(ctx); err != nil {
		return nil, fmt.Errorf("orchestrator: river migrate: %w", err)
	}

	// Enqueue: models × scenarios × repeats.
	for _, model := range models {
		for rep := 1; rep <= repeats; rep++ {
			jobID := fmt.Sprintf("bench-%s-r%d-%s", model, rep, time.Now().UTC().Format("20060102-150405"))
			if err := client.InsertBatch(ctx, scenarios, model, cfg.Provider, cfg.MCPServer, jobID, "", parallel); err != nil {
				return nil, fmt.Errorf("orchestrator: enqueue: %w", err)
			}
		}
	}
	log.Printf("[orchestrator] enqueued %d runs across %d workers", total, parallel)

	// Start workers.
	if err := client.Start(ctx); err != nil {
		return nil, fmt.Errorf("orchestrator: start workers: %w", err)
	}

	// Wait for completion.
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		c := atomic.LoadInt64(&completed)
		if c >= total {
			break
		}
		select {
		case <-ctx.Done():
			stopCtx, cancel := context.WithTimeout(context.Background(), config.GracefulStopTimeout)
			_ = client.Stop(stopCtx)
			cancel()
			return nil, ctx.Err()
		case <-ticker.C:
			p := atomic.LoadInt64(&passed)
			f := atomic.LoadInt64(&failed)
			log.Printf("[orchestrator] progress: %d/%d (pass=%d fail=%d)", c, total, p, f)
		}
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), config.GracefulStopTimeout)
	defer cancel()
	_ = client.Stop(stopCtx)

	return &RunResult{
		Total:     total,
		Completed: atomic.LoadInt64(&completed),
		Passed:    atomic.LoadInt64(&passed),
		Failed:    atomic.LoadInt64(&failed),
	}, nil
}
