package benchsvc

import (
	"errors"
	"net/http"
	"strconv"

	"samebits.com/evidra-infra-bench/internal/apiutil"
	"samebits.com/evidra-infra-bench/internal/auth"
	bench "samebits.com/evidra-infra-bench/pkg/bench"
)

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
			ScenarioID:        q.Get("scenario"),
			ScenarioIDs:       parseCSVQuery(q.Get("scenarios")),
			Model:             q.Get("model"),
			Provider:          q.Get("provider"),
			ToolServer:        q.Get("tool_server"),
			ToolServerVersion: q.Get("tool_server_version"),
			ReportID:          q.Get("report_id"),
			EvidenceMode:      q.Get("evidence_mode"),
			Since:             parseSince(q.Get("since")),
			Limit:             limit,
			Offset:            offset,
			SortBy:            q.Get("sort_by"),
			SortOrder:         q.Get("sort_order"),
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
