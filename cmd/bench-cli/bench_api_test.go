package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vitas/evidra-bench/internal/auth"
	"github.com/vitas/evidra-bench/internal/benchsvc"
)

func TestBenchSessionLoginStatusAndLogout(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	registerBenchAPIRoutes(mux, benchsvc.NewService(nil, benchsvc.ServiceConfig{}), "secret-key", "tenant-browser")

	loginRec := httptest.NewRecorder()
	loginReq := httptest.NewRequest("POST", "https://bench.example/v1/bench/session", strings.NewReader(`{"api_key":"secret-key"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200; body: %s", loginRec.Code, loginRec.Body.String())
	}
	sessionCookie := findCookie(loginRec.Result().Cookies(), auth.SessionCookieName)
	if sessionCookie == nil {
		t.Fatalf("login did not set %s cookie", auth.SessionCookieName)
	}
	if !sessionCookie.HttpOnly {
		t.Fatal("session cookie must be HttpOnly")
	}

	statusRec := httptest.NewRecorder()
	statusReq := httptest.NewRequest("GET", "https://bench.example/v1/bench/session", nil)
	statusReq.AddCookie(sessionCookie)
	mux.ServeHTTP(statusRec, statusReq)

	if statusRec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", statusRec.Code)
	}
	if !strings.Contains(statusRec.Body.String(), `"authenticated":true`) {
		t.Fatalf("status body = %s, want authenticated true", statusRec.Body.String())
	}
	if !strings.Contains(statusRec.Body.String(), `"tenant_id":"tenant-browser"`) {
		t.Fatalf("status body = %s, want tenant id", statusRec.Body.String())
	}

	logoutRec := httptest.NewRecorder()
	logoutReq := httptest.NewRequest("DELETE", "https://bench.example/v1/bench/session", nil)
	logoutReq.AddCookie(sessionCookie)
	mux.ServeHTTP(logoutRec, logoutReq)

	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want 204", logoutRec.Code)
	}
	clearCookie := findCookie(logoutRec.Result().Cookies(), auth.SessionCookieName)
	if clearCookie == nil || clearCookie.MaxAge >= 0 {
		t.Fatalf("logout should clear session cookie, got %#v", clearCookie)
	}
}

func TestBenchSessionLoginRejectsInvalidKey(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	registerBenchAPIRoutes(mux, benchsvc.NewService(nil, benchsvc.ServiceConfig{}), "secret-key", "tenant-browser")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "https://bench.example/v1/bench/session", strings.NewReader(`{"api_key":"wrong"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if findCookie(rec.Result().Cookies(), auth.SessionCookieName) != nil {
		t.Fatal("invalid login must not set session cookie")
	}
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}
