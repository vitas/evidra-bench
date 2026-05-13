package benchsvc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------- Models ----------

func TestHandleListModels_ReturnsModels(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		enabledModels: []EnabledModel{
			{
				ID:                "gemini-2.5-flash",
				DisplayName:       "Gemini 2.5 Flash",
				Provider:          "google",
				InputCostPerMtok:  0.15,
				OutputCostPerMtok: 0.60,
			},
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/models", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body struct {
		Models []EnabledModel `json:"models"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(body.Models))
	}
	if body.Models[0].ID != "gemini-2.5-flash" {
		t.Fatalf("id = %q, want gemini-2.5-flash", body.Models[0].ID)
	}
	if repo.lastTenant != "tenant-a" {
		t.Fatalf("tenant = %q, want tenant-a", repo.lastTenant)
	}
}

func TestHandleUpsertTenantProvider_Returns404WhileDisabled(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/v1/bench/models/gemini-2.5-flash/provider", strings.NewReader(`{"api_key":"sk-secret","rate_limit":10}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	if repo.lastTenant != "" {
		t.Fatalf("tenant = %q, want empty because route is disabled", repo.lastTenant)
	}
	if repo.lastModelID != "" {
		t.Fatalf("modelID = %q, want empty because route is disabled", repo.lastModelID)
	}
}

func TestHandleDeleteTenantProvider_Returns404WhileDisabled(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/v1/bench/models/gemini-2.5-flash/provider", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	if repo.lastTenant != "" {
		t.Fatalf("tenant = %q, want empty because route is disabled", repo.lastTenant)
	}
	if repo.lastModelID != "" {
		t.Fatalf("modelID = %q, want empty because route is disabled", repo.lastModelID)
	}
}

func TestHandleUpdateGlobalModel_Returns204(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	svc := NewService(repo, ServiceConfig{})
	handler := HandleUpdateGlobalModel(svc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/v1/admin/bench/models/gemini-2.5-flash", strings.NewReader(`{"api_key_env":"CUSTOM_KEY","api_base_url":"https://gw.example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("model_id", "gemini-2.5-flash")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if repo.lastModelID != "gemini-2.5-flash" {
		t.Fatalf("modelID = %q, want gemini-2.5-flash", repo.lastModelID)
	}
	if repo.lastGlobalCfg.APIKeyEnv != "CUSTOM_KEY" {
		t.Fatalf("api_key_env = %q, want CUSTOM_KEY", repo.lastGlobalCfg.APIKeyEnv)
	}
	if repo.lastGlobalCfg.APIBaseURL != "https://gw.example.com" {
		t.Fatalf("api_base_url = %q, want https://gw.example.com", repo.lastGlobalCfg.APIBaseURL)
	}
}
