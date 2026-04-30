//go:build integration

package benchsvc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListModels_PlatformDefault_And_TenantOverride(t *testing.T) {
	pool := setupTestDB(t)
	store := NewPgStore(pool)
	tenantID := testID("tnt")
	platformModelID := testID("gemini")
	privateModelID := testID("private")

	seedTenant(t, pool, tenantID)
	seedModelWithEnv(t, pool, platformModelID, "GEMINI_API_KEY")
	seedModel(t, pool, privateModelID)

	if err := store.UpsertTenantProvider(t.Context(), tenantID, privateModelID, TenantProviderConfig{
		APIKeyEnc: "sk-tenant-key",
	}); err != nil {
		t.Fatalf("UpsertTenantProvider: %v", err)
	}

	svc := NewService(store, ServiceConfig{})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth(tenantID))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/models", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Models []EnabledModel `json:"models"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Models) != 2 {
		t.Fatalf("len(models) = %d, want 2", len(resp.Models))
	}
}
