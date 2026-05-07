package benchsvc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	bench "samebits.com/evidra-infra-bench/pkg/bench"
)

// ---------- Ingest ----------

func TestHandleIngestRun_ValidPayload(t *testing.T) {
	t.Parallel()

	// IngestRun calls BeginTx which our handlerRepo doesn't support,
	// so we use a dedicated repo that returns a fakeTx.
	repo := &ingestRepo{}
	svc := NewService(repo, ServiceConfig{PublicTenant: "pub"})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("tenant-a"))

	payload := `{"id":"r1","scenario_id":"s1","model":"sonnet"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/bench/runs", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["ok"] != true {
		t.Fatalf("ok = %v, want true", body["ok"])
	}
}

func TestHandleIngestRun_RejectsMissingFields(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	tests := []struct {
		name    string
		payload string
	}{
		{"missing id", `{"scenario_id":"s1","model":"m1"}`},
		{"missing scenario_id", `{"id":"r1","model":"m1"}`},
		{"missing model", `{"id":"r1","scenario_id":"s1"}`},
		{"empty body", `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/bench/runs", strings.NewReader(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
		})
	}
}

func TestHandleIngestBatch_ImportsRuns(t *testing.T) {
	t.Parallel()

	repo := &ingestRepo{batchCount: 3}
	svc := NewService(repo, ServiceConfig{PublicTenant: "pub"})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("tenant-a"))

	payload := `{"runs":[
		{"id":"r1","scenario_id":"s1","model":"m1"},
		{"id":"r2","scenario_id":"s1","model":"m1"},
		{"id":"r3","scenario_id":"s2","model":"m2"}
	]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/bench/runs/batch", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["ok"] != true {
		t.Fatalf("ok = %v, want true", body["ok"])
	}
	if int(body["imported"].(float64)) != 3 {
		t.Fatalf("imported = %v, want 3", body["imported"])
	}
}

// ---------- Ingest support: fake that supports transactions ----------

// ingestRepo wraps handlerRepo with a fake transaction that accepts Exec and Commit.
type ingestRepo struct {
	handlerRepo
	batchCount int
}

func (r *ingestRepo) UpsertScenarios(_ context.Context, _ []bench.ScenarioSummary) (int, error) {
	return 0, nil
}
func (r *ingestRepo) BeginTx(_ context.Context) (pgx.Tx, error) {
	// Reuse fakeTx from service_batch_test.go (same package).
	// Supply enough "INSERT 0 1" tags so IngestRunBatch counts rows as inserted.
	tags := make([]pgconn.CommandTag, 20)
	for i := range tags {
		tags[i] = pgconn.NewCommandTag("INSERT 0 1")
	}
	return &fakeTx{execTags: tags}, nil
}

func (r *ingestRepo) InsertRunBatch(_ context.Context, _ string, _ []bench.RunRecord) (int, error) {
	return r.batchCount, nil
}
