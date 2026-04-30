package benchsvc

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"samebits.com/evidra-infra-bench/internal/apiutil"
	"samebits.com/evidra-infra-bench/internal/auth"
	bench "samebits.com/evidra-infra-bench/pkg/bench"
)

// RegisterRoutes adds bench intelligence routes to the given mux.
// Public routes (leaderboard, scenarios) are registered directly.
// Authenticated routes go through authMw and extract tenant from context.
func RegisterRoutes(mux *http.ServeMux, svc *Service, authMw func(http.Handler) http.Handler) {
	// Public — no auth.
	mux.HandleFunc("GET /v1/bench/leaderboard", handleLeaderboard(svc))

	// Authenticated — scenarios (metadata is IP).
	mux.Handle("GET /v1/bench/scenarios", authMw(http.HandlerFunc(handleListScenarios(svc))))

	// Authenticated — ingest.
	mux.Handle("POST /v1/bench/runs", authMw(http.HandlerFunc(handleIngestRun(svc))))
	mux.Handle("POST /v1/bench/runs/batch", authMw(http.HandlerFunc(handleIngestBatch(svc))))
	mux.Handle("POST /v1/bench/scenarios/sync", authMw(http.HandlerFunc(handleSyncScenarios(svc))))

	// Authenticated — delete / archive.
	mux.Handle("DELETE /v1/bench/runs/{id}", authMw(http.HandlerFunc(handleDeleteRun(svc))))
	mux.Handle("POST /v1/bench/runs/archive", authMw(http.HandlerFunc(handleArchiveRuns(svc))))

	// Authenticated — queries.
	mux.Handle("GET /v1/bench/runs", authMw(http.HandlerFunc(handleListRuns(svc))))
	mux.Handle("GET /v1/bench/runs/{id}", authMw(http.HandlerFunc(handleGetRun(svc))))
	mux.Handle("GET /v1/bench/runs/{id}/transcript", authMw(http.HandlerFunc(handleGetTranscript(svc))))
	mux.Handle("GET /v1/bench/runs/{id}/tool-calls", authMw(http.HandlerFunc(handleGetToolCalls(svc))))
	mux.Handle("GET /v1/bench/runs/{id}/timeline", authMw(http.HandlerFunc(handleGetTimeline(svc))))
	mux.Handle("GET /v1/bench/stats", authMw(http.HandlerFunc(handleStats(svc))))
	mux.Handle("GET /v1/bench/catalog", authMw(http.HandlerFunc(handleCatalog(svc))))
	mux.Handle("GET /v1/bench/models", authMw(http.HandlerFunc(handleListModels(svc))))
	// TODO: enable after adding AES-256-GCM key encryption (EVIDRA_ENCRYPTION_KEY).
	// Per-tenant API key storage is disabled until encryption is implemented.
	// mux.Handle("PUT /v1/bench/models/{model_id}/provider", authMw(http.HandlerFunc(handleUpsertTenantProvider(svc))))
	// mux.Handle("DELETE /v1/bench/models/{model_id}/provider", authMw(http.HandlerFunc(handleDeleteTenantProvider(svc))))
	mux.Handle("GET /v1/bench/runs/{id}/scorecard", authMw(http.HandlerFunc(handleGetScorecard(svc))))
	mux.Handle("GET /v1/bench/compare/runs", authMw(http.HandlerFunc(handleCompareRuns(svc))))
	mux.Handle("GET /v1/bench/compare/models", authMw(http.HandlerFunc(handleCompareModels(svc))))
	mux.Handle("GET /v1/bench/signals", authMw(http.HandlerFunc(handleSignals(svc))))
	mux.Handle("GET /v1/bench/regressions", authMw(http.HandlerFunc(handleRegressions(svc))))
	mux.Handle("GET /v1/bench/insights", authMw(http.HandlerFunc(handleFailureAnalysis(svc))))

	// Trigger routes — only enabled when TriggerStore is configured.
	if svc.cfg.TriggerStore != nil {
		mux.Handle("POST /v1/bench/trigger", authMw(http.HandlerFunc(handleTrigger(svc, svc.cfg.TriggerStore, svc.cfg.Executor))))
		mux.Handle("GET /v1/bench/trigger/{id}", authMw(http.HandlerFunc(handleTriggerStatus(svc.cfg.TriggerStore))))
		mux.Handle("POST /v1/bench/trigger/{id}/progress", authMw(http.HandlerFunc(handleTriggerProgress(svc, svc.cfg.TriggerStore))))
	}

	// Runner routes — V2b multi-runner support.
	mux.Handle("POST /v1/runners/register", authMw(http.HandlerFunc(handleRegisterRunner(svc))))
	mux.Handle("GET /v1/runners", authMw(http.HandlerFunc(handleListRunners(svc))))
	mux.Handle("DELETE /v1/runners/{id}", authMw(http.HandlerFunc(handleDeleteRunner(svc))))
	mux.Handle("GET /v1/runners/jobs", authMw(http.HandlerFunc(handlePollJob(svc))))
	mux.Handle("POST /v1/runners/jobs/{id}/complete", authMw(http.HandlerFunc(handleCompleteJob(svc))))
}

// parseSince parses a "since" query parameter as RFC3339 or date string.
// Returns nil if the string is empty or unparseable.
func parseSince(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, err = time.Parse("2006-01-02", s)
	}
	if err != nil {
		return nil
	}
	return &t
}

func handleLeaderboard(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mode := r.URL.Query().Get("evidence_mode")
		k := 3
		if kStr := r.URL.Query().Get("k"); kStr != "" {
			if kVal, err := strconv.Atoi(kStr); err == nil && kVal >= 1 && kVal <= 10 {
				k = kVal
			}
		}
		entries, err := svc.Leaderboard(r.Context(), mode, k)
		if err != nil {
			if errors.Is(err, ErrPublicTenantUnavailable) {
				apiutil.WriteError(w, http.StatusServiceUnavailable, err.Error())
				return
			}
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, map[string]any{
			"models":        entries,
			"evidence_mode": mode,
		})
	}
}

func handleIngestRun(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())

		var req IngestRunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if req.ID == "" || req.ScenarioID == "" || req.Model == "" {
			apiutil.WriteError(w, http.StatusBadRequest, "id, scenario_id, and model are required")
			return
		}
		req.TenantID = tenantID
		if err := svc.IngestRun(r.Context(), tenantID, req); err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, "insert: "+err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusCreated, map[string]any{"ok": true, "id": req.ID})
	}
}

func handleIngestBatch(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())

		var req struct {
			Runs []IngestRunRequest `json:"runs"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if len(req.Runs) == 0 {
			apiutil.WriteError(w, http.StatusBadRequest, "runs array is empty")
			return
		}
		for i := range req.Runs {
			if req.Runs[i].ID == "" || req.Runs[i].ScenarioID == "" || req.Runs[i].Model == "" {
				apiutil.WriteError(w, http.StatusBadRequest, "each run requires id, scenario_id, and model")
				return
			}
			req.Runs[i].TenantID = tenantID
		}
		count, err := svc.IngestRunBatch(r.Context(), tenantID, req.Runs)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, "batch insert: "+err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusCreated, map[string]any{
			"ok":       true,
			"imported": count,
			"total":    len(req.Runs),
		})
	}
}

func handleListRuns(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		if limit <= 0 {
			limit = 50
		}
		offset, _ := strconv.Atoi(q.Get("offset"))

		f := bench.RunFilters{
			ScenarioID:   q.Get("scenario"),
			Model:        q.Get("model"),
			Provider:     q.Get("provider"),
			EvidenceMode: q.Get("evidence_mode"),
			Since:        parseSince(q.Get("since")),
			Limit:        limit,
			Offset:       offset,
			SortBy:       q.Get("sort_by"),
			SortOrder:    q.Get("sort_order"),
		}
		if q.Get("passed") == "true" {
			f.PassedOnly = true
		}
		if q.Get("passed") == "false" {
			f.FailedOnly = true
		}
		if q.Get("exclude_errors") == "true" {
			f.ExcludeErrors = true
		}

		runs, total, err := svc.ListRuns(r.Context(), tenantID, f)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if runs == nil {
			runs = []bench.RunRecord{}
		}
		apiutil.WriteJSON(w, http.StatusOK, map[string]any{
			"runs":   runs,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
	}
}

func handleGetRun(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		run, err := svc.GetRun(r.Context(), tenantID, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteError(w, http.StatusNotFound, "run not found")
			} else {
				apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, run)
	}
}

func handleStats(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		q := r.URL.Query()
		f := bench.RunFilters{
			ScenarioID:   q.Get("scenario"),
			Model:        q.Get("model"),
			Provider:     q.Get("provider"),
			EvidenceMode: q.Get("evidence_mode"),
			Since:        parseSince(q.Get("since")),
		}
		st, err := svc.FilteredStats(r.Context(), tenantID, f)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, st)
	}
}

func handleGetTranscript(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		data, contentType, err := svc.GetArtifact(r.Context(), tenantID, id, "transcript")
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteError(w, http.StatusNotFound, "transcript not found")
				return
			}
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}
}

func handleGetToolCalls(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		data, contentType, err := svc.GetArtifact(r.Context(), tenantID, id, "tool_calls")
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteError(w, http.StatusNotFound, "tool calls not found")
				return
			}
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}
}

func handleGetTimeline(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		data, _, err := svc.GetArtifact(r.Context(), tenantID, id, "tool_calls")
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteError(w, http.StatusNotFound, "tool calls not found (needed for timeline)")
				return
			}
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}

		var calls []bench.ToolCall
		if err := json.Unmarshal(data, &calls); err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, "parse tool calls: "+err.Error())
			return
		}

		tl := bench.Parse(calls)
		apiutil.WriteJSON(w, http.StatusOK, tl)
	}
}

func handleCatalog(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		cat, err := svc.Catalog(r.Context(), tenantID)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, "catalog query failed")
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, cat)
	}
}

func handleGetScorecard(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		data, contentType, err := svc.GetArtifact(r.Context(), tenantID, id, "scorecard")
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteError(w, http.StatusNotFound, "scorecard not found")
				return
			}
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}
}

func handleListModels(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		models, err := svc.ListEnabledModels(r.Context(), tenantID)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}

		type modelResponse struct {
			ID                string  `json:"id"`
			DisplayName       string  `json:"display_name"`
			Provider          string  `json:"provider"`
			APIBaseURL        string  `json:"api_base_url,omitempty"`
			Available         bool    `json:"available"`
			InputCostPerMtok  float64 `json:"input_cost_per_mtok"`
			OutputCostPerMtok float64 `json:"output_cost_per_mtok"`
		}

		result := make([]modelResponse, 0, len(models))
		for _, m := range models {
			result = append(result, modelResponse{
				ID:                m.ID,
				DisplayName:       m.DisplayName,
				Provider:          m.Provider,
				APIBaseURL:        m.APIBaseURL,
				Available:         os.Getenv(m.APIKeyEnv) != "",
				InputCostPerMtok:  m.InputCostPerMtok,
				OutputCostPerMtok: m.OutputCostPerMtok,
			})
		}
		apiutil.WriteJSON(w, http.StatusOK, map[string]any{"models": result})
	}
}

// HandleUpdateGlobalModel updates platform-level defaults for a model.
// This handler is intended to be wrapped by an invite-secret gate in the API router.
func HandleUpdateGlobalModel(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		modelID := r.PathValue("model_id")

		var cfg GlobalModelConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			apiutil.WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if err := svc.UpdateGlobalModel(r.Context(), modelID, cfg); err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleListScenarios(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scenarios, err := svc.ListScenarios(r.Context())
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if scenarios == nil {
			scenarios = []bench.ScenarioSummary{}
		}
		apiutil.WriteJSON(w, http.StatusOK, map[string]any{
			"scenarios": scenarios,
		})
	}
}

func handleSyncScenarios(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Scenarios []bench.ScenarioSummary `json:"scenarios"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if len(req.Scenarios) == 0 {
			apiutil.WriteError(w, http.StatusBadRequest, "scenarios array is required")
			return
		}
		upserted, err := svc.UpsertScenarios(r.Context(), req.Scenarios)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"upserted": upserted,
			"total":    len(req.Scenarios),
		})
	}
}

func handleCompareRuns(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		a := r.URL.Query().Get("a")
		b := r.URL.Query().Get("b")
		if a == "" || b == "" {
			apiutil.WriteError(w, http.StatusBadRequest, "query params 'a' and 'b' (run IDs) are required")
			return
		}
		result, err := svc.CompareRuns(r.Context(), tenantID, a, b)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteError(w, http.StatusNotFound, "one or both runs not found")
			} else {
				apiutil.WriteError(w, http.StatusInternalServerError, "comparison failed")
			}
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, result)
	}
}

func handleCompareModels(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		q := r.URL.Query()

		// Support both ?models=X,Y,Z (multi-model matrix) and legacy ?a=X&b=Y (pairwise).
		modelsStr := q.Get("models")
		if modelsStr != "" {
			models := strings.Split(modelsStr, ",")
			var scenarios []string
			if scenariosStr := q.Get("scenarios"); scenariosStr != "" {
				scenarios = strings.Split(scenariosStr, ",")
			}
			mode := q.Get("evidence_mode")
			matrix, err := svc.ModelMatrix(r.Context(), tenantID, models, scenarios, mode)
			if err != nil {
				apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
				return
			}
			apiutil.WriteJSON(w, http.StatusOK, matrix)
			return
		}

		// Legacy pairwise comparison.
		modelA := q.Get("a")
		modelB := q.Get("b")
		if modelA == "" || modelB == "" {
			apiutil.WriteError(w, http.StatusBadRequest, "query param 'models' (comma-separated) or 'a' and 'b' are required")
			return
		}
		mode := q.Get("evidence_mode")
		result, err := svc.CompareModels(r.Context(), tenantID, modelA, modelB, mode)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, result)
	}
}

func handleSignals(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		q := r.URL.Query()
		f := bench.RunFilters{
			ScenarioID:   q.Get("scenario"),
			Model:        q.Get("model"),
			Provider:     q.Get("provider"),
			EvidenceMode: q.Get("evidence_mode"),
			Since:        parseSince(q.Get("since")),
		}

		agg, err := svc.SignalSummary(r.Context(), tenantID, f)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, agg)
	}
}

func handleRegressions(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		regs, err := svc.Regressions(r.Context(), tenantID)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if regs == nil {
			regs = []bench.Regression{}
		}
		apiutil.WriteJSON(w, http.StatusOK, regs)
	}
}

func handleFailureAnalysis(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		scenario := r.URL.Query().Get("scenario")
		if scenario == "" {
			apiutil.WriteError(w, http.StatusBadRequest, "query param 'scenario' is required")
			return
		}

		insights, err := svc.FailureAnalysis(r.Context(), tenantID, scenario)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, insights)
	}
}

func handleDeleteRun(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		err := svc.DeleteRun(r.Context(), tenantID, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteError(w, http.StatusNotFound, "run not found")
			} else {
				apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleArchiveRuns(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())

		var req ArchiveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if req.Before == nil && len(req.IDs) == 0 && req.Model == "" {
			apiutil.WriteError(w, http.StatusBadRequest, "at least one filter is required: before, ids, or model")
			return
		}

		count, err := svc.ArchiveRuns(r.Context(), tenantID, req)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, map[string]any{"archived": count})
	}
}
