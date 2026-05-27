package benchsvc

import (
	"net/http"
	"strings"
	"time"

	"github.com/vitas/evidra-bench/internal/apiutil"
	"github.com/vitas/evidra-bench/internal/auth"
)

// RegisterRoutes adds bench intelligence routes to the given mux.
// Read routes resolve a tenant at the backend boundary: authenticated requests
// use authMw, while unauthenticated reads fall back to the configured public
// tenant. Mutating, configuration, trigger, and runner routes require authMw.
func RegisterRoutes(mux *http.ServeMux, svc *Service, authMw func(http.Handler) http.Handler) {
	readMw := readTenantMiddleware(svc, authMw)

	// Read endpoints.
	mux.Handle("GET /v1/bench/leaderboard", readMw(http.HandlerFunc(handleLeaderboard(svc))))
	mux.Handle("GET /v1/bench/scenarios", readMw(http.HandlerFunc(handleListScenarios(svc))))
	mux.Handle("GET /v1/bench/scenario-improvements", readMw(http.HandlerFunc(handleListScenarioImprovements(svc))))
	mux.Handle("GET /v1/bench/review-candidates", readMw(http.HandlerFunc(handleListReviewCandidates(svc))))
	mux.Handle("GET /v1/bench/runs", readMw(http.HandlerFunc(handleListRuns(svc))))
	mux.Handle("GET /v1/bench/runs/{id}", readMw(http.HandlerFunc(handleGetRun(svc))))
	mux.Handle("GET /v1/bench/runs/{id}/transcript", readMw(http.HandlerFunc(handleGetTranscript(svc))))
	mux.Handle("GET /v1/bench/runs/{id}/tool-calls", readMw(http.HandlerFunc(handleGetToolCalls(svc))))
	mux.Handle("GET /v1/bench/runs/{id}/timeline", readMw(http.HandlerFunc(handleGetTimeline(svc))))
	mux.Handle("GET /v1/bench/runs/{id}/scorecard", readMw(http.HandlerFunc(handleGetScorecard(svc))))
	mux.Handle("GET /v1/bench/runs/{id}/autopsy", readMw(http.HandlerFunc(handleGetAutopsy(svc))))
	mux.Handle("GET /v1/bench/runs/{id}/run-error", readMw(http.HandlerFunc(handleGetRunError(svc))))
	mux.Handle("GET /v1/bench/runs/{id}/run-events", readMw(http.HandlerFunc(handleGetRunEvents(svc))))
	mux.Handle("GET /v1/bench/runs/{id}/review", readMw(http.HandlerFunc(handleGetRunReview(svc))))
	mux.Handle("GET /v1/bench/runs/{id}/scenario-patch-preview", readMw(http.HandlerFunc(handleGetScenarioPatchPreview(svc))))
	mux.Handle("GET /v1/bench/runs/{id}/scenario-patch.diff", readMw(http.HandlerFunc(handleGetScenarioPatchDiff(svc))))
	mux.Handle("GET /v1/bench/runs/{id}/scenario-patch-validation", readMw(http.HandlerFunc(handleGetScenarioPatchValidation(svc, svc.cfg.TriggerStore))))
	mux.Handle("GET /v1/bench/stats", readMw(http.HandlerFunc(handleStats(svc))))
	mux.Handle("GET /v1/bench/catalog", readMw(http.HandlerFunc(handleCatalog(svc))))
	mux.Handle("GET /v1/bench/compare/runs", readMw(http.HandlerFunc(handleCompareRuns(svc))))
	mux.Handle("GET /v1/bench/compare/models", readMw(http.HandlerFunc(handleCompareModels(svc))))
	mux.Handle("GET /v1/bench/compare/tool-server", readMw(http.HandlerFunc(handleCompareToolServer(svc))))
	mux.Handle("GET /v1/bench/reports/tool-server", readMw(http.HandlerFunc(handleToolServerReport(svc))))
	mux.Handle("GET /v1/bench/reports/tool-server-matrix", readMw(http.HandlerFunc(handleToolServerMatrixReport(svc))))
	mux.Handle("GET /v1/bench/signals", readMw(http.HandlerFunc(handleSignals(svc))))
	mux.Handle("GET /v1/bench/regressions", readMw(http.HandlerFunc(handleRegressions(svc))))
	mux.Handle("GET /v1/bench/insights", readMw(http.HandlerFunc(handleFailureAnalysis(svc))))

	// Authenticated — ingest.
	mux.Handle("POST /v1/bench/runs", authMw(http.HandlerFunc(handleIngestRun(svc))))
	mux.Handle("POST /v1/bench/runs/batch", authMw(http.HandlerFunc(handleIngestBatch(svc))))
	mux.Handle("POST /v1/bench/scenarios/sync", authMw(http.HandlerFunc(handleSyncScenarios(svc))))
	mux.Handle("POST /v1/bench/review-candidates/{id}/draft", authMw(http.HandlerFunc(handlePostRunReviewDraft(svc))))
	mux.Handle("POST /v1/bench/runs/{id}/review-draft", authMw(http.HandlerFunc(handlePostRunReviewDraft(svc))))
	mux.Handle("POST /v1/bench/runs/{id}/scenario-patch-preview", authMw(http.HandlerFunc(handlePostScenarioPatchPreview(svc))))
	mux.Handle("PUT /v1/bench/runs/{id}/review", authMw(http.HandlerFunc(handlePutRunReview(svc))))

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
		mux.Handle("POST /v1/bench/runs/{id}/scenario-patch-validation", authMw(http.HandlerFunc(handlePostScenarioPatchValidation(svc, svc.cfg.TriggerStore, svc.cfg.Executor))))
	}

	// Runner routes — V2b multi-runner support.
	mux.Handle("POST /v1/runners/register", authMw(http.HandlerFunc(handleRegisterRunner(svc))))
	mux.Handle("GET /v1/runners", authMw(http.HandlerFunc(handleListRunners(svc))))
	mux.Handle("DELETE /v1/runners/{id}", authMw(http.HandlerFunc(handleDeleteRunner(svc))))
	mux.Handle("GET /v1/runners/jobs", authMw(http.HandlerFunc(handlePollJob(svc))))
	mux.Handle("POST /v1/runners/jobs/{id}/complete", authMw(http.HandlerFunc(handleCompleteJob(svc))))
}

func readTenantMiddleware(svc *Service, authMw func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.TrimSpace(r.Header.Get("Authorization")) != "" || hasSessionCookie(r) {
				authMw(next).ServeHTTP(w, r)
				return
			}
			if svc.cfg.PublicTenant == "" {
				apiutil.WriteError(w, http.StatusServiceUnavailable, ErrPublicTenantUnavailable.Error())
				return
			}
			ctx := auth.WithTenantID(r.Context(), svc.cfg.PublicTenant)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func hasSessionCookie(r *http.Request) bool {
	cookie, err := r.Cookie(auth.SessionCookieName)
	return err == nil && strings.TrimSpace(cookie.Value) != ""
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
