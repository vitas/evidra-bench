package benchsvc

import (
	"net/http"

	"samebits.com/evidra-infra-bench/internal/apiutil"
	"samebits.com/evidra-infra-bench/internal/auth"
	bench "samebits.com/evidra-infra-bench/pkg/bench"
)

func handleStats(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		q := r.URL.Query()
		f := bench.RunFilters{
			ScenarioID:        q.Get("scenario"),
			Model:             q.Get("model"),
			Provider:          q.Get("provider"),
			ToolServer:        q.Get("tool_server"),
			ToolServerVersion: q.Get("tool_server_version"),
			ReportID:          q.Get("report_id"),
			EvidenceMode:      q.Get("evidence_mode"),
			Since:             parseSince(q.Get("since")),
		}
		st, err := svc.FilteredStats(r.Context(), tenantID, f)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, st)
	}
}

func handleSignals(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		q := r.URL.Query()
		f := bench.RunFilters{
			ScenarioID:        q.Get("scenario"),
			Model:             q.Get("model"),
			Provider:          q.Get("provider"),
			ToolServer:        q.Get("tool_server"),
			ToolServerVersion: q.Get("tool_server_version"),
			ReportID:          q.Get("report_id"),
			EvidenceMode:      q.Get("evidence_mode"),
			Since:             parseSince(q.Get("since")),
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
