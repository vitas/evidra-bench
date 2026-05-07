package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"samebits.com/evidra-infra-bench/pkg/config"
)

func TestHandleCertifyAPI_RejectsUnsupportedEvidenceMode(t *testing.T) {
	t.Parallel()

	handler := handleCertifyAPI(config.Default(), newNoopParallelRunner(), t.TempDir())
	req := httptest.NewRequest(http.MethodPost, "/v1/certify", strings.NewReader(`{"config":{"evidence_mode":"legacy"}}`))
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "unsupported evidence_mode") {
		t.Fatalf("body = %q, want unsupported evidence_mode", rec.Body.String())
	}
}

func TestServeCertifyParallel_RejectsNonDefaultSharedProfiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestScenario(t, dir, "kubernetes", "s-default", nil)
	writeTestScenarioWithProfile(t, dir, "kubernetes", "s-argocd", "argocd")

	cfg := config.Default()
	cfg.ScenariosDir = dir
	cfg.EnvironmentProvider = "kind"

	runner := newNoopParallelRunner()
	handler := handleCertifyAPI(cfg, runner, t.TempDir())

	body := `{"model":"sonnet","scenarios":["s-default","s-argocd"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/certify", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "argocd") {
		t.Fatalf("body = %q, want argocd rejection", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "shared-cluster parallel") {
		t.Fatalf("body = %q, want shared-cluster parallel mention", rec.Body.String())
	}
	// Runner should not be called; give a short window to confirm.
	select {
	case <-runner.done:
		t.Fatal("runner should not be called when profile validation fails")
	case <-time.After(500 * time.Millisecond):
		// expected
	}
}

func TestHandleCertifyAPI_FiltersIncompatibleScenarios(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestScenario(t, dir, "kubernetes", "s-kind", []string{"kind"})
	writeTestScenario(t, dir, "kubernetes", "s-k3d", []string{"k3d"})
	writeTestScenario(t, dir, "kubernetes", "s-all", nil)

	cfg := config.Default()
	cfg.ScenariosDir = dir
	cfg.EnvironmentProvider = "kind"

	runner := newNoopParallelRunner()
	handler := handleCertifyAPI(cfg, runner, t.TempDir())

	body := `{"model":"sonnet","scenarios":["s-kind","s-k3d","s-all"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/certify", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body = %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// s-k3d should be skipped, leaving 2 scenarios.
	if resp["total"] != "2" {
		t.Fatalf("total = %q, want 2", resp["total"])
	}
	if resp["skipped"] != "1" {
		t.Fatalf("skipped = %q, want 1", resp["skipped"])
	}

	runner.waitCalled(t)

	if len(runner.scenarios) == 0 {
		t.Fatal("runner was not called")
	}
	// Verify only compatible scenario paths were passed to runner.
	if len(runner.scenarios) != 2 {
		t.Fatalf("runner got %d scenarios, want 2", len(runner.scenarios))
	}
}

func TestHandleCertifyAPI_RejectsFullyIncompatibleRequest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestScenario(t, dir, "kubernetes", "s-k3d-only", []string{"k3d"})

	cfg := config.Default()
	cfg.ScenariosDir = dir
	cfg.EnvironmentProvider = "kind"

	runner := newNoopParallelRunner()
	handler := handleCertifyAPI(cfg, runner, t.TempDir())

	body := `{"model":"sonnet","scenarios":["s-k3d-only"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/certify", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "incompatible") {
		t.Fatalf("body = %q, want incompatible error", rec.Body.String())
	}
	select {
	case <-runner.done:
		t.Fatal("runner should not be called when all scenarios are incompatible")
	case <-time.After(500 * time.Millisecond):
		// expected
	}
}
