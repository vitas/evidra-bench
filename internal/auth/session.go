package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const SessionCookieName = "bench_session"

type sessionClaims struct {
	TenantID  string `json:"tenant_id"`
	ExpiresAt int64  `json:"exp"`
}

// NewSessionCookie returns a signed HttpOnly browser session cookie.
func NewSessionCookie(r *http.Request, apiKey, tenantID string, ttl time.Duration) (*http.Cookie, error) {
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	expires := time.Now().Add(ttl)
	value, err := newSessionCookieValue(apiKey, tenantID, expires)
	if err != nil {
		return nil, err
	}
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secureCookieRequest(r),
	}, nil
}

// ClearSessionCookie returns a Set-Cookie value that expires the browser session.
func ClearSessionCookie(r *http.Request) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secureCookieRequest(r),
	}
}

func newSessionCookieValue(apiKey, tenantID string, expiresAt time.Time) (string, error) {
	if strings.TrimSpace(apiKey) == "" {
		return "", fmt.Errorf("api key is required")
	}
	if strings.TrimSpace(tenantID) == "" {
		return "", fmt.Errorf("tenant id is required")
	}
	payload, err := json.Marshal(sessionClaims{
		TenantID:  tenantID,
		ExpiresAt: expiresAt.Unix(),
	})
	if err != nil {
		return "", err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	signature := signSessionPayload(apiKey, encodedPayload)
	return encodedPayload + "." + signature, nil
}

func verifySessionCookieValue(apiKey, value string, now time.Time) (string, bool) {
	if strings.TrimSpace(apiKey) == "" {
		return "", false
	}
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return "", false
	}
	wantSig := signSessionPayload(apiKey, parts[0])
	if !hmac.Equal([]byte(parts[1]), []byte(wantSig)) {
		return "", false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	var claims sessionClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", false
	}
	if strings.TrimSpace(claims.TenantID) == "" || claims.ExpiresAt <= now.Unix() {
		return "", false
	}
	return claims.TenantID, true
}

func signSessionPayload(apiKey, encodedPayload string) string {
	mac := hmac.New(sha256.New, []byte(apiKey))
	_, _ = mac.Write([]byte(encodedPayload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func secureCookieRequest(r *http.Request) bool {
	if r != nil && r.TLS != nil {
		return true
	}
	if r != nil && strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
		return true
	}
	return false
}
