package benchsvc

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/vitas/evidra-bench/internal/apiutil"
	"github.com/vitas/evidra-bench/internal/auth"
	"github.com/vitas/evidra-bench/pkg/artifact"
	bench "github.com/vitas/evidra-bench/pkg/bench"
	"github.com/vitas/evidra-bench/pkg/runreview"
)

func handleGetTranscript(svc *Service) http.HandlerFunc {
	return handleGetArtifact(svc, artifact.HostedTranscript, "transcript not found")
}

func handleGetToolCalls(svc *Service) http.HandlerFunc {
	return handleGetArtifact(svc, artifact.HostedToolCalls, "tool calls not found")
}

func handleGetTimeline(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")

		if data, contentType, err := svc.GetArtifact(r.Context(), tenantID, id, artifact.HostedTimeline); err == nil {
			writeRawArtifactResponse(w, data, contentType)
			return
		} else if !errors.Is(err, ErrNotFound) {
			writeArtifactReadError(w, err, "")
			return
		}

		data, _, err := svc.GetArtifact(r.Context(), tenantID, id, artifact.HostedToolCalls)
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

func handleGetScorecard(svc *Service) http.HandlerFunc {
	return handleGetArtifact(svc, artifact.HostedScorecard, "scorecard not found")
}

func handleGetAutopsy(svc *Service) http.HandlerFunc {
	return handleGetArtifact(svc, artifact.HostedFailureAutopsy, "failure autopsy not found")
}

func handleGetRunError(svc *Service) http.HandlerFunc {
	return handleGetArtifact(svc, artifact.HostedRunError, "run error not found")
}

func handleGetRunEvents(svc *Service) http.HandlerFunc {
	return handleGetArtifact(svc, artifact.HostedRunEvents, "run events not found")
}

func handleGetRunReview(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		data, _, err := svc.GetArtifact(r.Context(), tenantID, id, artifact.HostedRunReview)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteError(w, http.StatusNotFound, "run review not found")
				return
			}
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		review, err := runreview.Decode(data)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, "parse run review: "+err.Error())
			return
		}
		if strings.TrimSpace(r.Header.Get("Authorization")) == "" && !runreview.IsPublic(review) {
			apiutil.WriteError(w, http.StatusNotFound, "run review not found")
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, review)
	}
}

func handlePutRunReview(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		run, err := svc.GetRun(r.Context(), tenantID, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteError(w, http.StatusNotFound, "run not found")
				return
			}
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		var review runreview.Review
		if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
			apiutil.WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		defaultVisibility := runreview.VisibilityPrivate
		if tenantID != "" && tenantID == svc.cfg.PublicTenant {
			defaultVisibility = runreview.VisibilityPublic
		}
		normalized, err := runreview.NormalizeForRun(review, id, run.ScenarioID, defaultVisibility)
		if err != nil {
			apiutil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		data, err := runreview.Marshal(normalized)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, "marshal run review: "+err.Error())
			return
		}
		if err := svc.StoreArtifact(r.Context(), id, artifact.HostedRunReview, artifact.ContentTypeJSON, data); err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, normalized)
	}
}

func handleGetArtifact(svc *Service, artifactType, notFoundMessage string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		data, contentType, err := svc.GetArtifact(r.Context(), tenantID, id, artifactType)
		if err != nil {
			writeArtifactReadError(w, err, notFoundMessage)
			return
		}
		writeRawArtifactResponse(w, data, contentType)
	}
}

func writeRawArtifactResponse(w http.ResponseWriter, data []byte, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func writeArtifactReadError(w http.ResponseWriter, err error, notFoundMessage string) {
	if errors.Is(err, ErrNotFound) {
		apiutil.WriteError(w, http.StatusNotFound, notFoundMessage)
		return
	}
	apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
}
