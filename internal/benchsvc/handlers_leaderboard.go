package benchsvc

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"samebits.com/evidra-infra-bench/internal/apiutil"
)

func handleLeaderboard(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mode := r.URL.Query().Get("evidence_mode")
		k := 3
		if kStr := r.URL.Query().Get("k"); kStr != "" {
			if kVal, err := strconv.Atoi(kStr); err == nil && kVal >= 1 && kVal <= 10 {
				k = kVal
			}
		}
		scenarios := parseCSVQuery(r.URL.Query().Get("scenarios"))
		entries, err := svc.Leaderboard(r.Context(), mode, k, scenarios)
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

func parseCSVQuery(value string) []string {
	if value == "" {
		return nil
	}
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, raw := range strings.Split(value, ",") {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
