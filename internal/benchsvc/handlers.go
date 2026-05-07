package benchsvc

import (
	"net/http"
	"time"

	"samebits.com/evidra-infra-bench/internal/apiutil"
	"samebits.com/evidra-infra-bench/internal/auth"
)

// RegisterRoutes adds bench intelligence routes to the given mux.
// Public routes (leaderboard, scenarios) are registered directly.
// Authenticated routes go through authMw and extract tenant from context.
func RegisterRoutes(mux *http.ServeMux, svc *Service, authMw func(http.Handler) http.Handler) {
	publicReadMw := publicTenantMiddleware(svc)

	// Public — no auth.
	mux.HandleFunc("GET /v1/bench/leaderboard", handleLeaderboard(svc))
	mux.Handle("GET /v1/bench/scenarios", publicReadMw(http.HandlerFunc(handleListScenarios(svc))))
	mux.Handle("GET /v1/bench/runs", publicReadMw(http.HandlerFunc(handleListRuns(svc))))
	mux.Handle("GET /v1/bench/runs/{id}", publicReadMw(http.HandlerFunc(handleGetRun(svc))))
	mux.Handle("GET /v1/bench/runs/{id}/transcript", publicReadMw(http.HandlerFunc(handleGetTranscript(svc))))
	mux.Handle("GET /v1/bench/runs/{id}/tool-calls", publicReadMw(http.HandlerFunc(handleGetToolCalls(svc))))
	mux.Handle("GET /v1/bench/runs/{id}/timeline", publicReadMw(http.HandlerFunc(handleGetTimeline(svc))))
	mux.Handle("GET /v1/bench/runs/{id}/scorecard", publicReadMw(http.HandlerFunc(handleGetScorecard(svc))))
	mux.Handle("GET /v1/bench/runs/{id}/autopsy", publicReadMw(http.HandlerFunc(handleGetAutopsy(svc))))
	mux.Handle("GET /v1/bench/stats", publicReadMw(http.HandlerFunc(handleStats(svc))))
	mux.Handle("GET /v1/bench/catalog", publicReadMw(http.HandlerFunc(handleCatalog(svc))))
	mux.Handle("GET /v1/bench/compare/runs", publicReadMw(http.HandlerFunc(handleCompareRuns(svc))))
	mux.Handle("GET /v1/bench/compare/models", publicReadMw(http.HandlerFunc(handleCompareModels(svc))))
	mux.Handle("GET /v1/bench/signals", publicReadMw(http.HandlerFunc(handleSignals(svc))))
	mux.Handle("GET /v1/bench/regressions", publicReadMw(http.HandlerFunc(handleRegressions(svc))))
	mux.Handle("GET /v1/bench/insights", publicReadMw(http.HandlerFunc(handleFailureAnalysis(svc))))

	// Authenticated — ingest.
	mux.Handle("POST /v1/bench/runs", authMw(http.HandlerFunc(handleIngestRun(svc))))
	mux.Handle("POST /v1/bench/runs/batch", authMw(http.HandlerFunc(handleIngestBatch(svc))))
	mux.Handle("POST /v1/bench/scenarios/sync", authMw(http.HandlerFunc(handleSyncScenarios(svc))))

	// Authenticated — delete / archive.
	mux.Handle("DELETE /v1/bench/runs/{id}", authMw(http.HandlerFunc(handleDeleteRun(svc))))
	mux.Handle("POST /v1/bench/runs/archive", authMw(http.HandlerFunc(handleArchiveRuns(svc))))

	// Authenticated — model provider configuration.
	mux.Handle("GET /v1/bench/models", authMw(http.HandlerFunc(handleListModels(svc))))
	// TODO: enable after adding AES-256-GCM key encryption (BENCH_ENCRYPTION_KEY).
	// Per-tenant API key storage is disabled until encryption is implemented.
	// mux.Handle("PUT /v1/bench/models/{model_id}/provider", authMw(http.HandlerFunc(handleUpsertTenantProvider(svc))))
	// mux.Handle("DELETE /v1/bench/models/{model_id}/provider", authMw(http.HandlerFunc(handleDeleteTenantProvider(svc))))

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

func publicTenantMiddleware(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if svc.cfg.PublicTenant == "" {
				apiutil.WriteError(w, http.StatusServiceUnavailable, ErrPublicTenantUnavailable.Error())
				return
			}
			ctx := auth.WithTenantID(r.Context(), svc.cfg.PublicTenant)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
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
