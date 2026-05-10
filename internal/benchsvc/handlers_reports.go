package benchsvc

import (
	"net/http"

	"samebits.com/evidra-infra-bench/internal/apiutil"
	"samebits.com/evidra-infra-bench/internal/auth"
)

func handleToolServerReport(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		q := r.URL.Query()
		model := q.Get("model")
		toolServer := q.Get("tool_server")
		if model == "" || toolServer == "" {
			apiutil.WriteError(w, http.StatusBadRequest, "query params 'model' and 'tool_server' are required")
			return
		}

		scenarios := parseCSVQuery(q.Get("scenarios"))
		if scenario := q.Get("scenario"); scenario != "" {
			scenarios = parseCSVQuery(scenario)
		}

		report, err := svc.BuildToolServerReport(r.Context(), tenantID, ToolServerReportRequest{
			Model:             model,
			ToolServer:        toolServer,
			ToolServerVersion: q.Get("tool_server_version"),
			Category:          q.Get("category"),
			ScenarioIDs:       scenarios,
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
			_, _ = w.Write([]byte(RenderToolServerReportMarkdown(report)))
		default:
			apiutil.WriteError(w, http.StatusBadRequest, "format must be json or markdown")
		}
	}
}
