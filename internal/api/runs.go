package api

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"samebits.com/evidra-infra-bench/pkg/store"
)

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	offset, _ := strconv.Atoi(q.Get("offset"))

	f := store.RunFilters{
		ScenarioID: q.Get("scenario"),
		Model:      q.Get("model"),
		Provider:   q.Get("provider"),
		Since:      q.Get("since"),
		Limit:      limit,
		Offset:     offset,
	}
	if q.Get("passed") == "true" {
		f.PassedOnly = true
	}
	if q.Get("passed") == "false" {
		f.FailedOnly = true
	}

	runs, total, err := s.store.ListRuns(r.Context(), f)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if runs == nil {
		runs = []store.RunRecord{}
	}

	respondJSON(w, http.StatusOK, listResponse{
		Items:  runs,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := s.store.GetRun(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "run not found")
		return
	}
	respondJSON(w, http.StatusOK, run)
}

func (s *Server) handleGetTranscript(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := s.store.GetRun(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "run not found")
		return
	}
	s.serveArtifactFile(w, run.ArtifactDir, "transcript.txt", "text/plain")
}

func (s *Server) handleGetToolCalls(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := s.store.GetRun(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "run not found")
		return
	}
	s.serveArtifactFile(w, run.ArtifactDir, "tool-calls.json", "application/json")
}

func (s *Server) handleGetScorecard(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := s.store.GetRun(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "run not found")
		return
	}
	s.serveArtifactFile(w, run.ArtifactDir, "scorecard.json", "application/json")
}

func (s *Server) serveArtifactFile(w http.ResponseWriter, artifactDir, filename, contentType string) {
	if artifactDir == "" {
		respondError(w, http.StatusNotFound, "no artifact directory")
		return
	}
	path := filepath.Join(artifactDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		respondError(w, http.StatusNotFound, filename+" not found")
		return
	}
	w.Header().Set("Content-Type", contentType)
	if _, err := w.Write(data); err != nil {
		log.Printf("[api] write artifact %s: %v", filename, err)
	}
}
