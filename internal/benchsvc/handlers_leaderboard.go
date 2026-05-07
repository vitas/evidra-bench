package benchsvc

import (
	"errors"
	"net/http"
	"strconv"

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
