package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServeAuthMiddleware_UsesSharedBearerParsing(t *testing.T) {
	t.Parallel()

	handler := authMiddleware("secret-token", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/certify", nil)
	req.Header.Set("Authorization", "  bearer secret-token  ")
	handler(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
}
