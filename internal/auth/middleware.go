package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"math/big"
	"net/http"
	"strings"
	"time"
)

const bearerPrefix = "bearer "

// StaticKeyMiddleware authenticates requests using a single static API key.
// Sets tenant ID to defaultTenant for all authenticated requests.
func StaticKeyMiddleware(apiKey, defaultTenant string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID, ok := AuthenticateStaticRequest(r, apiKey, defaultTenant)
			if !ok {
				authFail(w)
				return
			}
			ctx := WithTenantID(r.Context(), tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AuthenticateStaticRequest validates either a Bearer API key or a signed
// browser session cookie and returns the authenticated tenant.
func AuthenticateStaticRequest(r *http.Request, apiKey, defaultTenant string) (string, bool) {
	token := ParseBearerToken(r.Header.Get("Authorization"))
	if StaticKeyMatches(apiKey, token) {
		return defaultTenant, true
	}
	if strings.TrimSpace(apiKey) == "" {
		return "", false
	}
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return "", false
	}
	tenantID, ok := verifySessionCookieValue(apiKey, cookie.Value, time.Now())
	if !ok {
		return "", false
	}
	if defaultTenant != "" && tenantID != defaultTenant {
		return "", false
	}
	return tenantID, true
}

// StaticKeyMatches compares a candidate API key against the configured key.
func StaticKeyMatches(apiKey, candidate string) bool {
	if strings.TrimSpace(apiKey) == "" || candidate == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(apiKey)) == 1
}

func extractBearerToken(header string) string {
	return ParseBearerToken(header)
}

// ParseBearerToken extracts a bearer token using case-insensitive scheme matching.
func ParseBearerToken(header string) string {
	trimmed := strings.TrimSpace(header)
	if !strings.HasPrefix(strings.ToLower(trimmed), bearerPrefix) {
		return ""
	}
	return trimmed[len(bearerPrefix):]
}

func authFail(w http.ResponseWriter) {
	jitterSleep()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
}

// jitterSleep adds 50-100ms random delay to prevent timing attacks.
func jitterSleep() {
	n, err := rand.Int(rand.Reader, big.NewInt(50))
	if err != nil {
		time.Sleep(75 * time.Millisecond)
		return
	}
	time.Sleep(time.Duration(50+n.Int64()) * time.Millisecond)
}

// KeyStoreAuthFunc is a function that looks up a key and returns a tenant ID.
type KeyStoreAuthFunc func(ctx context.Context, plaintext string) (tenantID string, err error)

// KeyStoreMiddleware authenticates requests using a database-backed key store.
func KeyStoreMiddleware(lookup KeyStoreAuthFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := ParseBearerToken(r.Header.Get("Authorization"))
			if token == "" {
				authFail(w)
				return
			}
			tenantID, err := lookup(r.Context(), token)
			if err != nil {
				authFail(w)
				return
			}
			ctx := WithTenantID(r.Context(), tenantID)
			w.Header().Set("X-Bench-Tenant", tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AuthCheckHandler returns 200 with X-Bench-Tenant for valid tokens (forwardAuth target).
func AuthCheckHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tid := TenantID(r.Context())
		if tid == "" {
			authFail(w)
			return
		}
		w.Header().Set("X-Bench-Tenant", tid)
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"ok": "true", "tenant_id": tid})
	}
}

// ExtractTenantHeader reads X-Bench-Tenant for downstream use (e.g., after forwardAuth).
func ExtractTenantHeader(header string) string {
	return strings.TrimSpace(header)
}
