package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vitas/evidra-bench/internal/benchsvc"
	"github.com/vitas/evidra-bench/pkg/config"
)

func TestRegisterBenchAPIRoutes_ProtectsWriteEndpoint(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	svc := benchsvc.NewService(nil, benchsvc.ServiceConfig{})
	registerBenchAPIRoutes(mux, svc, "secret")

	tests := []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/v1/bench/runs"},
		{method: http.MethodPost, path: "/v1/bench/runs/run-1/review-draft"},
		{method: http.MethodPut, path: "/v1/bench/runs/run-1/review"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, tt.path, nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want 401; body = %s", tt.method, tt.path, rec.Code, rec.Body.String())
		}
	}
}

func TestRegisterBenchAPIRoutes_InfoIsPublicJSON(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	svc := benchsvc.NewService(nil, benchsvc.ServiceConfig{})
	registerBenchAPIRoutes(mux, svc, "secret")

	req := httptest.NewRequest(http.MethodGet, "/v1/bench/info", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Readonly bool   `json:"readonly"`
		Version  string `json:"version"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode info response: %v", err)
	}
	if body.Readonly {
		t.Fatal("readonly = true, want false")
	}
	if body.Version == "" {
		t.Fatal("version is empty")
	}
}

func TestPrepareServeRunner_ControlPlaneOnlySkipsProvision(t *testing.T) {
	t.Parallel()

	fake := &fakeServeOrchestrator{}

	runner, teardown, err := prepareServeRunner(context.Background(), config.Config{}, serveOptions{ControlPlaneOnly: true}, func(config.Config) serveOrchestrator {
		return fake
	})
	if err != nil {
		t.Fatalf("prepareServeRunner returned error: %v", err)
	}
	if runner != nil {
		t.Fatal("expected no direct runner in control-plane-only mode")
	}
	if teardown == nil {
		t.Fatal("expected no-op teardown")
	}
	teardown(context.Background())
	if fake.provisionCalls != 0 {
		t.Fatalf("expected Provision to be skipped, got %d calls", fake.provisionCalls)
	}
	if fake.teardownCalls != 0 {
		t.Fatalf("expected Teardown to be skipped, got %d calls", fake.teardownCalls)
	}
}

func TestHandleCertifyDisabled_ReturnsNotImplemented(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/v1/certify", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	handleCertifyDisabled().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d", http.StatusNotImplemented, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "direct executor disabled") {
		t.Fatalf("expected direct executor disabled response, got %s", rec.Body.String())
	}
}
