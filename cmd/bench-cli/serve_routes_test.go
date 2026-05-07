package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"samebits.com/evidra-infra-bench/internal/benchsvc"
	"samebits.com/evidra-infra-bench/pkg/config"
)

func TestRegisterBenchAPIRoutes_ProtectsWriteEndpoint(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	svc := benchsvc.NewService(nil, benchsvc.ServiceConfig{})
	registerBenchAPIRoutes(mux, svc, "secret")

	req := httptest.NewRequest(http.MethodPost, "/v1/bench/runs", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body = %s", rec.Code, rec.Body.String())
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
