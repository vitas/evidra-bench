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

	"samebits.com/evidra-infra-bench/internal/apiutil"
	"samebits.com/evidra-infra-bench/internal/auth"
)

var errPinnedRunnerUnavailable = errors.New("pinned runner unavailable")

// handleTrigger returns a handler that starts a new bench trigger job.
// POST /v1/bench/trigger
func handleTrigger(svc *Service, store *TriggerStore, executor RunExecutor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())

		req, ok := decodeTriggerRequest(w, r)
		if !ok {
			return
		}

		handled, ok := maybeHandleRunnerTrigger(w, r, svc, store, tenantID, req)
		if handled {
			return
		}
		if !ok {
			return
		}

		if executor == nil {
			apiutil.WriteError(w, http.StatusNotImplemented, "bench trigger not configured: no executor")
			return
		}

		provider, ok := resolveDirectTriggerProvider(w, r, svc, req)
		if !ok {
			return
		}

		job := &TriggerJob{
			ID:                NewJobID(),
			Status:            "pending",
			Model:             req.Model,
			Provider:          provider,
			EvidenceMode:      req.EvidenceMode,
			ExecutionMode:     req.ExecutionMode,
			MCPServer:         req.MCPServer,
			ToolServer:        req.ToolServer,
			ToolServerVersion: req.ToolServerVersion,
			Total:             len(req.Scenarios),
			Progress:          pendingScenarioProgress(req.Scenarios),
			CreatedAt:         time.Now(),
		}
		store.Create(job)

		benchURL := resolveBenchURL(r)
		apiKey := r.Header.Get("Authorization")

		go func() {
			if err := executor.Start(context.Background(), job, benchURL, apiKey); err != nil {
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

		apiutil.WriteJSON(w, http.StatusAccepted, map[string]any{
			"id":     job.ID,
			"status": job.Status,
		})
	}
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
	evidenceMode, validEvidenceMode := normalizeTriggerEvidenceMode(req)
	if !validEvidenceMode {
		apiutil.WriteError(w, http.StatusBadRequest, "evidence_mode must be none or mcp")
		return TriggerRequest{}, false
	}
	req.EvidenceMode = evidenceMode
	var ok bool
	req.ExecutionMode, ok = normalizeTriggerExecutionMode(req.ExecutionMode)
	if !ok {
		apiutil.WriteError(w, http.StatusBadRequest, "execution_mode must be provider or a2a")
		return TriggerRequest{}, false
	}
	return req, true
}

func isSupportedTriggerEvidenceMode(mode string) bool {
	return mode == "none" || mode == "mcp"
}

func normalizeTriggerEvidenceMode(req TriggerRequest) (string, bool) {
	switch {
	case req.EvidenceMode != "":
		return req.EvidenceMode, isSupportedTriggerEvidenceMode(req.EvidenceMode)
	case req.MCPServer != "" || req.ToolServer != "":
		return "mcp", true
	default:
		return "none", true
	}
}

func normalizeTriggerExecutionMode(mode string) (string, bool) {
	switch mode {
	case "", "provider":
		return "provider", true
	case "a2a":
		return "a2a", true
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

func maybeHandleRunnerTrigger(w http.ResponseWriter, r *http.Request, svc *Service, store *TriggerStore, tenantID string, req TriggerRequest) (handled bool, ok bool) {
	if svc.cfg.Dispatcher == nil {
		return false, true
	}

	runner, err := resolveRunnerForTrigger(r.Context(), svc, tenantID, req)
	if err != nil {
		switch {
		case errors.Is(err, errPinnedRunnerUnavailable):
			apiutil.WriteError(w, http.StatusBadRequest, err.Error())
		default:
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
		}
		return true, false
	}
	if runner == nil {
		return false, true
	}

	provider, ok := resolveRunnerTriggerProvider(w, r, svc, req, runner)
	if !ok {
		return true, false
	}

	jobID, err := enqueueRunnerTrigger(r.Context(), svc, store, tenantID, req, provider, runner)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
		return true, false
	}

	apiutil.WriteJSON(w, http.StatusAccepted, map[string]any{
		"id":     jobID,
		"status": "pending",
		"mode":   "runner",
	})
	return true, true
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

	runner, err := svc.repo.FindRunnerForModel(ctx, tenantID, req.Model)
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
		EvidenceMode:      req.EvidenceMode,
		ExecutionMode:     req.ExecutionMode,
		MCPServer:         req.MCPServer,
		ToolServer:        req.ToolServer,
		ToolServerVersion: req.ToolServerVersion,
	}
	benchJob, err := svc.repo.EnqueueJob(ctx, tenantID, req.Model, provider, cfg)
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
		EvidenceMode:      req.EvidenceMode,
		ExecutionMode:     req.ExecutionMode,
		MCPServer:         req.MCPServer,
		ToolServer:        req.ToolServer,
		ToolServerVersion: req.ToolServerVersion,
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
func handleTriggerStatus(store *TriggerStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		job := store.Get(id)
		if job == nil {
			apiutil.WriteError(w, http.StatusNotFound, "job not found")
			return
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
			apiutil.WriteError(w, http.StatusNotFound, "job not found")
			return
		}

		// Update bench_jobs for persistence and janitor tracking.
		// Read accumulated passed/failed from TriggerStore (it tracks per-scenario status).
		current := store.Get(update.JobID)
		passed, failed := 0, 0
		if current != nil {
			passed = current.Passed
			failed = current.Failed
		}
		_ = svc.repo.UpdateJobProgress(r.Context(), update.JobID, update.Completed, passed, failed)

		w.WriteHeader(http.StatusOK)
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
