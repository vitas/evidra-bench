package benchsvc

import (
	"net/http"

	"samebits.com/evidra-infra-bench/internal/apiutil"
	"samebits.com/evidra-infra-bench/internal/auth"
)

func handleToolServerMatrixReport(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		q := r.URL.Query()
		model := q.Get("model")
		reportID := q.Get("report_id")
		toolServers := parseCSVQuery(q.Get("tool_servers"))
		toolServerVersions := parseCSVQuery(q.Get("tool_server_versions"))
		if model == "" || reportID == "" || len(toolServers) == 0 {
			apiutil.WriteError(w, http.StatusBadRequest, "query params 'model', 'report_id', and 'tool_servers' are required")
			return
		}
		if len(toolServerVersions) > 0 && len(toolServerVersions) != len(toolServers) {
			apiutil.WriteError(w, http.StatusBadRequest, "tool_server_versions must match tool_servers length")
			return
		}

		scenarios := parseCSVQuery(q.Get("scenarios"))
		if scenario := q.Get("scenario"); scenario != "" {
			scenarios = parseCSVQuery(scenario)
		}

		report, err := svc.BuildToolServerMatrixReport(r.Context(), tenantID, ToolServerMatrixReportRequest{
			Model:              model,
			ReportID:           reportID,
			ToolServers:        toolServers,
			ToolServerVersions: toolServerVersions,
			ScenarioIDs:        scenarios,
		})
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}

		switch q.Get("format") {
		case "", "json":
			apiutil.WriteJSON(w, http.StatusOK, report)
		case "markdown":
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(RenderToolServerMatrixReportMarkdown(report)))
		default:
			apiutil.WriteError(w, http.StatusBadRequest, "format must be json or markdown")
		}
	}
}
