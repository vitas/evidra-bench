package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExtractBearerToken(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"valid", "Bearer abc123", "abc123"},
		{"no prefix", "abc123", ""},
		{"empty", "", ""},
		{"lowercase", "bearer abc123", "abc123"},
		{"mixed case", "bEaReR abc123", "abc123"},
		{"extra spaces", "Bearer  abc123", " abc123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractBearerToken(tt.header)
			if got != tt.want {
				t.Errorf("extractBearerToken(%q) = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}

func TestStaticKeyMiddleware_ValidKey(t *testing.T) {
	t.Parallel()
	handler := StaticKeyMiddleware("test-key", "default-tenant")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tid := TenantID(r.Context())
			_, _ = w.Write([]byte(tid))
		}),
	)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "default-tenant" {
		t.Fatalf("expected tenant=default-tenant, got %s", rec.Body.String())
	}
}

func TestStaticKeyMiddleware_NoAuth(t *testing.T) {
	t.Parallel()
	handler := StaticKeyMiddleware("test-key", "t1")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 401 {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestKeyStoreMiddleware_PassesRequestContext(t *testing.T) {
	t.Parallel()

	type ctxKey string

	handler := KeyStoreMiddleware(func(ctx context.Context, plaintext string) (string, error) {
		if plaintext != "lookup-key" {
			t.Fatalf("lookup token = %q, want %q", plaintext, "lookup-key")
		}
		if got, _ := ctx.Value(ctxKey("request_id")).(string); got != "req-123" {
			t.Fatalf("lookup context value = %q, want %q", got, "req-123")
		}
		return "tenant-123", nil
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(TenantID(r.Context())))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxKey("request_id"), "req-123"))
	req.Header.Set("Authorization", "Bearer lookup-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "tenant-123" {
		t.Fatalf("tenant = %q, want %q", rec.Body.String(), "tenant-123")
	}
	if rec.Header().Get("X-Bench-Tenant") != "tenant-123" {
		t.Fatalf("X-Bench-Tenant = %q, want tenant-123", rec.Header().Get("X-Bench-Tenant"))
	}
	if rec.Header().Get("X-Evidra-Tenant") != "" {
		t.Fatalf("X-Evidra-Tenant should not be emitted")
	}
}

func TestStaticKeyMiddleware_WrongKey(t *testing.T) {
	t.Parallel()
	handler := StaticKeyMiddleware("correct-key", "t1")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 401 {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestStaticKeyMiddleware_ValidSessionCookie(t *testing.T) {
	t.Parallel()
	handler := StaticKeyMiddleware("test-key", "tenant-browser")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(TenantID(r.Context())))
		}),
	)

	loginReq := httptest.NewRequest("POST", "https://bench.example/v1/bench/session", nil)
	cookie, err := NewSessionCookie(loginReq, "test-key", "tenant-browser", time.Hour)
	if err != nil {
		t.Fatalf("NewSessionCookie: %v", err)
	}
	if !cookie.HttpOnly {
		t.Fatal("session cookie must be HttpOnly")
	}
	if !cookie.Secure {
		t.Fatal("session cookie must be Secure on HTTPS")
	}

	req := httptest.NewRequest("PUT", "https://bench.example/v1/bench/runs/r1/review", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "tenant-browser" {
		t.Fatalf("tenant = %q, want tenant-browser", rec.Body.String())
	}
}

func TestStaticKeyMiddleware_ExpiredSessionCookie(t *testing.T) {
	t.Parallel()
	handler := StaticKeyMiddleware("test-key", "tenant-browser")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)

	value, err := newSessionCookieValue("test-key", "tenant-browser", time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatalf("newSessionCookieValue: %v", err)
	}
	req := httptest.NewRequest("PUT", "https://bench.example/v1/bench/runs/r1/review", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: value})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestStaticKeyMiddleware_RejectsSessionCookieWhenAPIKeyDisabled(t *testing.T) {
	t.Parallel()
	handler := StaticKeyMiddleware("", "tenant-browser")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)

	value := signSessionPayload("", "payload")
	req := httptest.NewRequest("PUT", "https://bench.example/v1/bench/runs/r1/review", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "payload." + value})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestStaticKeyMiddleware_RejectsSessionCookieForAnotherTenant(t *testing.T) {
	t.Parallel()
	handler := StaticKeyMiddleware("test-key", "tenant-browser")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)

	value, err := newSessionCookieValue("test-key", "tenant-other", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("newSessionCookieValue: %v", err)
	}
	req := httptest.NewRequest("PUT", "https://bench.example/v1/bench/runs/r1/review", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: value})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
