package benchsvc

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vitas/evidra-bench/internal/apiutil"
	"github.com/vitas/evidra-bench/internal/auth"
	"github.com/vitas/evidra-bench/pkg/artifact"
	bench "github.com/vitas/evidra-bench/pkg/bench"
)

func handleGetTranscript(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		data, contentType, err := svc.GetArtifact(r.Context(), tenantID, id, artifact.HostedTranscript)
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
		data, contentType, err := svc.GetArtifact(r.Context(), tenantID, id, artifact.HostedToolCalls)
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

		if data, contentType, err := svc.GetArtifact(r.Context(), tenantID, id, artifact.HostedTimeline); err == nil {
			w.Header().Set("Content-Type", contentType)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
			return
		} else if !errors.Is(err, ErrNotFound) {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
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
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		data, contentType, err := svc.GetArtifact(r.Context(), tenantID, id, artifact.HostedScorecard)
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

func handleGetAutopsy(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		data, contentType, err := svc.GetArtifact(r.Context(), tenantID, id, artifact.HostedFailureAutopsy)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteError(w, http.StatusNotFound, "failure autopsy not found")
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

func handleGetRunError(svc *Service) http.HandlerFunc {
	return handleGetJSONArtifact(svc, artifact.HostedRunError, "run error not found")
}

func handleGetRunEvents(svc *Service) http.HandlerFunc {
	return handleGetJSONArtifact(svc, artifact.HostedRunEvents, "run events not found")
}

func handleGetJSONArtifact(svc *Service, artifactType, notFoundMessage string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		data, contentType, err := svc.GetArtifact(r.Context(), tenantID, id, artifactType)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteError(w, http.StatusNotFound, notFoundMessage)
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
