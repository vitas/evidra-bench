package benchsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/vitas/evidra-bench/internal/apiutil"
	"github.com/vitas/evidra-bench/internal/auth"
)

var errPinnedRunnerUnavailable = errors.New("pinned runner unavailable")

type triggerStartResult struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Mode   string `json:"mode,omitempty"`
}

// handleTrigger returns a handler that starts a new bench trigger job.
// POST /v1/bench/trigger
func handleTrigger(svc *Service, store *TriggerStore, executor RunExecutor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())

		req, ok := decodeTriggerRequest(w, r)
		if !ok {
			return
		}

		result, ok := startTriggerRequest(w, r, svc, store, executor, tenantID, req)
		if !ok {
			return
		}

		apiutil.WriteJSON(w, http.StatusAccepted, result)
	}
}

func startTriggerRequest(w http.ResponseWriter, r *http.Request, svc *Service, store *TriggerStore, executor RunExecutor, tenantID string, req TriggerRequest) (triggerStartResult, bool) {
	handled, result, ok := maybeStartRunnerTrigger(w, r, svc, store, tenantID, req)
	if handled {
		return result, ok
	}
	if !ok {
		return triggerStartResult{}, false
	}

	if executor == nil {
		apiutil.WriteError(w, http.StatusNotImplemented, "bench trigger not configured: no executor")
		return triggerStartResult{}, false
	}

	provider, ok := resolveDirectTriggerProvider(w, r, svc, req)
	if !ok {
		return triggerStartResult{}, false
	}

	job := &TriggerJob{
		ID:                NewJobID(),
		Status:            "pending",
		Model:             req.Model,
		Provider:          provider,
		ExecutionMode:     req.ExecutionMode,
		MCPServer:         req.MCPServer,
		ToolServer:        req.ToolServer,
		ToolServerVersion: req.ToolServerVersion,
		SkillFile:         req.SkillFile,
		SkillID:           req.SkillID,
		SkillVersion:      req.SkillVersion,
		SkillSource:       req.SkillSource,
		SkillSHA256:       req.SkillSHA256,
		Total:             len(req.Scenarios),
		Progress:          pendingScenarioProgress(req.Scenarios),
		CreatedAt:         time.Now(),
	}
	store.Create(job)

	benchURL := resolveBenchURL(r)
	apiKey := r.Header.Get("Authorization")
	runCtx := svc.cfg.BackgroundContext

	go func() {
		if err := executor.Start(runCtx, job, benchURL, apiKey); err != nil {
			log.Printf("[bench-trigger] executor failed for job %s: %v", job.ID, err)
			store.Update(ProgressUpdate{
				JobID:     job.ID,
				Scenario:  "",
				Status:    "error",
				Completed: job.Total,
				Total:     job.Total,
			})
		}
	}()

	return triggerStartResult{
		ID:     job.ID,
		Status: job.Status,
	}, true
}

func decodeTriggerRequest(w http.ResponseWriter, r *http.Request) (TriggerRequest, bool) {
	var req TriggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return TriggerRequest{}, false
	}
	if req.Model == "" {
		apiutil.WriteError(w, http.StatusBadRequest, "model is required")
		return TriggerRequest{}, false
	}
	if len(req.Scenarios) == 0 {
		apiutil.WriteError(w, http.StatusBadRequest, "scenarios is required")
		return TriggerRequest{}, false
	}
	var ok bool
	req.ExecutionMode, ok = normalizeTriggerExecutionMode(req.ExecutionMode)
	if !ok {
		apiutil.WriteError(w, http.StatusBadRequest, "execution_mode must be provider or a2a")
		return TriggerRequest{}, false
	}
	return req, true
}

func normalizeTriggerExecutionMode(mode string) (string, bool) {
	switch mode {
	case "", ExecutionModeProvider:
		return ExecutionModeProvider, true
	case ExecutionModeA2A:
		return ExecutionModeA2A, true
	default:
		return "", false
	}
}

func resolveDirectTriggerProvider(w http.ResponseWriter, r *http.Request, svc *Service, req TriggerRequest) (string, bool) {
	provider := req.Provider
	info, err := svc.ResolveModelProvider(r.Context(), req.Model)
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "unknown model: "+req.Model)
		return "", false
	}
	if provider == "" {
		provider = info.Provider
	}
	if info.APIKeyEnv != "" && os.Getenv(info.APIKeyEnv) == "" {
		apiutil.WriteError(w, http.StatusBadRequest, "no API key configured for model: "+req.Model)
		return "", false
	}
	return provider, true
}

func maybeStartRunnerTrigger(w http.ResponseWriter, r *http.Request, svc *Service, store *TriggerStore, tenantID string, req TriggerRequest) (handled bool, result triggerStartResult, ok bool) {
	if svc.cfg.Dispatcher == nil {
		return false, triggerStartResult{}, true
	}

	runner, err := resolveRunnerForTrigger(r.Context(), svc, tenantID, req)
	if err != nil {
		switch {
		case errors.Is(err, errPinnedRunnerUnavailable):
			apiutil.WriteError(w, http.StatusBadRequest, err.Error())
		default:
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
		}
		return true, triggerStartResult{}, false
	}
	if runner == nil {
		return false, triggerStartResult{}, true
	}

	provider, ok := resolveRunnerTriggerProvider(w, r, svc, req, runner)
	if !ok {
		return true, triggerStartResult{}, false
	}

	jobID, err := enqueueRunnerTrigger(r.Context(), svc, store, tenantID, req, provider, runner)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
		return true, triggerStartResult{}, false
	}

	return true, triggerStartResult{
		ID:     jobID,
		Status: "pending",
		Mode:   "runner",
	}, true
}

func resolveRunnerTriggerProvider(w http.ResponseWriter, r *http.Request, svc *Service, req TriggerRequest, runner *Runner) (string, bool) {
	if req.Provider != "" {
		return req.Provider, true
	}
	if runner.Config.Provider != "" {
		return runner.Config.Provider, true
	}
	info, err := svc.ResolveModelProvider(r.Context(), req.Model)
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "unknown model: "+req.Model)
		return "", false
	}
	return info.Provider, true
}

func resolveRunnerForTrigger(ctx context.Context, svc *Service, tenantID string, req TriggerRequest) (*Runner, error) {
	if req.RunnerID != "" {
		runner, err := findPinnedRunner(ctx, svc, tenantID, req.RunnerID, req.Model)
		if err != nil {
			return nil, err
		}
		if runner == nil {
			return nil, fmt.Errorf("%w: runner %s is not available for model %s", errPinnedRunnerUnavailable, req.RunnerID, req.Model)
		}
		return runner, nil
	}

	runner, err := svc.repos.Jobs.FindRunnerForModel(ctx, tenantID, req.Model)
	if err != nil {
		return nil, fmt.Errorf("runner lookup failed: %w", err)
	}
	return runner, nil
}

func findPinnedRunner(ctx context.Context, svc *Service, tenantID, runnerID, model string) (*Runner, error) {
	runners, err := svc.ListRunners(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("runner list failed: %w", err)
	}
	for i := range runners {
		if runners[i].ID != runnerID || runners[i].Status != "healthy" {
			continue
		}
		for _, candidate := range runners[i].Config.Models {
			if candidate == model {
				return &runners[i], nil
			}
		}
	}
	return nil, nil
}

func enqueueRunnerTrigger(ctx context.Context, svc *Service, store *TriggerStore, tenantID string, req TriggerRequest, provider string, runner *Runner) (string, error) {
	cfg := JobConfig{
		Scenarios:         req.Scenarios,
		RunnerID:          req.RunnerID,
		ExecutionMode:     req.ExecutionMode,
		MCPServer:         req.MCPServer,
		ToolServer:        req.ToolServer,
		ToolServerVersion: req.ToolServerVersion,
		SkillFile:         req.SkillFile,
		SkillID:           req.SkillID,
		SkillVersion:      req.SkillVersion,
		SkillSource:       req.SkillSource,
		SkillSHA256:       req.SkillSHA256,
	}
	benchJob, err := svc.repos.Jobs.EnqueueJob(ctx, tenantID, req.Model, provider, cfg)
	if err != nil {
		return "", fmt.Errorf("enqueue job: %w", err)
	}
	if err := svc.cfg.Dispatcher.Dispatch(ctx, benchJob, runner); err != nil {
		log.Printf("[bench-trigger] dispatcher failed for job %s: %v", benchJob.ID, err)
	}

	store.Create(&TriggerJob{
		ID:                benchJob.ID,
		Status:            "pending",
		Model:             req.Model,
		Provider:          provider,
		ExecutionMode:     req.ExecutionMode,
		MCPServer:         req.MCPServer,
		ToolServer:        req.ToolServer,
		ToolServerVersion: req.ToolServerVersion,
		SkillFile:         req.SkillFile,
		SkillID:           req.SkillID,
		SkillVersion:      req.SkillVersion,
		SkillSource:       req.SkillSource,
		SkillSHA256:       req.SkillSHA256,
		Total:             len(req.Scenarios),
		Progress:          pendingScenarioProgress(req.Scenarios),
		CreatedAt:         time.Now(),
	})
	return benchJob.ID, nil
}

func pendingScenarioProgress(scenarios []string) []ScenarioProgress {
	progress := make([]ScenarioProgress, len(scenarios))
	for i, scenario := range scenarios {
		progress[i] = ScenarioProgress{Scenario: scenario, Status: "pending"}
	}
	return progress
}

// handleTriggerStatus returns a handler for GET /v1/bench/trigger/{id}.
// Supports SSE streaming when the client accepts it, otherwise returns a JSON snapshot.
func handleTriggerStatus(svc *Service, store *TriggerStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		job := store.Get(id)
		if job == nil {
			var err error
			job, err = restoreTriggerJobFromPersistentJob(r.Context(), svc, store, tenantID, id)
			if err != nil {
				apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if job == nil {
				apiutil.WriteError(w, http.StatusNotFound, "job not found")
				return
			}
		}

		// Check if SSE is possible.
		flusher, ok := w.(http.Flusher)
		if !ok || r.Header.Get("Accept") != "text/event-stream" {
			apiutil.WriteJSON(w, http.StatusOK, job)
			return
		}

		// SSE mode.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		// Send current state.
		if err := writeSSE(w, "status", job); err != nil {
			return
		}
		flusher.Flush()

		// If already terminal, close immediately.
		if job.Status == "completed" || job.Status == "failed" {
			if err := writeSSE(w, "complete", job); err != nil {
				return
			}
			flusher.Flush()
			return
		}

		ch := store.Subscribe(id)
		defer store.Unsubscribe(id, ch)

		for {
			select {
			case <-r.Context().Done():
				return
			case update, open := <-ch:
				if !open {
					return
				}
				if err := writeSSE(w, "progress", update); err != nil {
					return
				}
				flusher.Flush()

				// Check if job is done.
				current := store.Get(id)
				if current != nil && (current.Status == "completed" || current.Status == "failed") {
					if err := writeSSE(w, "complete", current); err != nil {
						return
					}
					flusher.Flush()
					return
				}
			}
		}
	}
}

// handleTriggerProgress returns a handler for POST /v1/bench/trigger/{id}/progress.
// This is the webhook endpoint called by the bench service.
func handleTriggerProgress(svc *Service, store *TriggerStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var update ProgressUpdate
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			apiutil.WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if update.ContractVersion == "" {
			// Accept for backward compatibility, but future versions will require it.
		} else if update.ContractVersion != ExecutorContractVersion {
			apiutil.WriteError(w, http.StatusBadRequest, "unsupported contract version")
			return
		}
		update.JobID = id

		if !store.Update(update) {
			tenantID := auth.TenantID(r.Context())
			job, err := restoreTriggerJobFromPersistentJob(r.Context(), svc, store, tenantID, id)
			if err != nil {
				apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if job == nil || !store.Update(update) {
				apiutil.WriteError(w, http.StatusNotFound, "job not found")
				return
			}
		}

		// Update bench_jobs for persistence and janitor tracking.
		// Read accumulated passed/failed from TriggerStore (it tracks per-scenario status).
		current := store.Get(update.JobID)
		passed, failed := 0, 0
		if current != nil {
			passed = current.Passed
			failed = current.Failed
		}
		_ = svc.repos.Jobs.UpdateJobProgress(r.Context(), update.JobID, update.Completed, passed, failed)

		w.WriteHeader(http.StatusOK)
	}
}

func restoreTriggerJobFromPersistentJob(ctx context.Context, svc *Service, store *TriggerStore, tenantID, jobID string) (*TriggerJob, error) {
	if svc == nil || svc.repos.Jobs == nil {
		return nil, nil
	}
	benchJob, err := svc.GetJob(ctx, tenantID, jobID)
	if err != nil {
		return nil, err
	}
	if benchJob == nil {
		return nil, nil
	}
	job := triggerJobFromBenchJob(benchJob)
	store.Create(job)
	return job, nil
}

func triggerJobFromBenchJob(job *BenchJob) *TriggerJob {
	var cfg JobConfig
	if len(job.ConfigJSON) > 0 {
		_ = json.Unmarshal(job.ConfigJSON, &cfg)
	}

	total := job.Total
	if total == 0 && len(cfg.Scenarios) > 0 {
		total = len(cfg.Scenarios)
	}
	executionMode := cfg.ExecutionMode
	if executionMode == "" {
		executionMode = ExecutionModeProvider
	}

	toolServer := job.ToolServer
	if toolServer == "" {
		toolServer = cfg.ToolServer
	}
	toolServerVersion := job.ToolServerVersion
	if toolServerVersion == "" {
		toolServerVersion = cfg.ToolServerVersion
	}
	skillID := job.SkillID
	if skillID == "" {
		skillID = cfg.SkillID
	}
	skillVersion := job.SkillVersion
	if skillVersion == "" {
		skillVersion = cfg.SkillVersion
	}

	return &TriggerJob{
		ID:                job.ID,
		Status:            triggerStatusFromBenchJobStatus(job.Status),
		Model:             job.Model,
		Provider:          job.Provider,
		ExecutionMode:     executionMode,
		MCPServer:         cfg.MCPServer,
		ToolServer:        toolServer,
		ToolServerVersion: toolServerVersion,
		SkillFile:         cfg.SkillFile,
		SkillID:           skillID,
		SkillVersion:      skillVersion,
		SkillSource:       cfg.SkillSource,
		SkillSHA256:       cfg.SkillSHA256,
		Total:             total,
		Completed:         job.Completed,
		Passed:            job.Passed,
		Failed:            job.Failed,
		Progress:          scenarioProgressFromBenchJob(cfg.Scenarios, job.Passed, job.Failed),
		CreatedAt:         job.CreatedAt,
	}
}

func scenarioProgressFromBenchJob(scenarios []string, passed, failed int) []ScenarioProgress {
	progress := pendingScenarioProgress(scenarios)
	for i := range progress {
		switch {
		case passed > 0:
			progress[i].Status = "passed"
			passed--
		case failed > 0:
			progress[i].Status = "failed"
			failed--
		default:
			return progress
		}
	}
	return progress
}

func triggerStatusFromBenchJobStatus(status string) string {
	switch status {
	case "", "queued":
		return "pending"
	case "claimed", "running":
		return "running"
	case "completed", "failed":
		return status
	default:
		return status
	}
}

// resolveBenchURL determines the base URL for the Bench API from the request.
// In container deployments, BENCH_SELF_URL overrides request-based resolution
// because external and internal URLs may differ.
func resolveBenchURL(r *http.Request) string {
	if selfURL := os.Getenv("BENCH_SELF_URL"); selfURL != "" {
		return selfURL
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}
	return fmt.Sprintf("%s://%s", scheme, r.Host)
}

// writeSSE writes a server-sent event to the response writer.
func writeSSE(w http.ResponseWriter, event string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
	return err
}
