package agent

import (
	"net/http"
	"testing"
	"time"
)

func TestIsRetryable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		code     int
		expected bool
	}{
		{200, false},
		{400, false},
		{401, false},
		{429, true},
		{500, true},
		{502, true},
		{503, true},
		{504, true},
	}
	for _, tt := range tests {
		if got := IsRetryable(tt.code); got != tt.expected {
			t.Errorf("IsRetryable(%d) = %v, want %v", tt.code, got, tt.expected)
		}
	}
}

func TestBackoffDuration_Exponential(t *testing.T) {
	t.Parallel()
	cfg := RetryConfig{InitialBackoff: 1 * time.Second, MaxBackoff: 30 * time.Second, Multiplier: 2.0}
	h := http.Header{}

	d0 := BackoffDuration(cfg, 0, h)
	if d0 != 1*time.Second {
		t.Fatalf("attempt 0: expected 1s, got %s", d0)
	}
	d1 := BackoffDuration(cfg, 1, h)
	if d1 != 2*time.Second {
		t.Fatalf("attempt 1: expected 2s, got %s", d1)
	}
	d2 := BackoffDuration(cfg, 2, h)
	if d2 != 4*time.Second {
		t.Fatalf("attempt 2: expected 4s, got %s", d2)
	}
}

func TestBackoffDuration_MaxCap(t *testing.T) {
	t.Parallel()
	cfg := RetryConfig{InitialBackoff: 1 * time.Second, MaxBackoff: 5 * time.Second, Multiplier: 10.0}
	d := BackoffDuration(cfg, 3, http.Header{})
	if d != 5*time.Second {
		t.Fatalf("expected max 5s, got %s", d)
	}
}

func TestBackoffDuration_RetryAfterHeader(t *testing.T) {
	t.Parallel()
	cfg := DefaultRetryConfig()
	h := http.Header{}
	h.Set("Retry-After", "10")
	d := BackoffDuration(cfg, 0, h)
	if d != 10*time.Second {
		t.Fatalf("expected 10s from header, got %s", d)
	}
}

func TestBackoffDuration_RetryAfterHeader_Capped(t *testing.T) {
	t.Parallel()
	cfg := RetryConfig{InitialBackoff: 1 * time.Second, MaxBackoff: 5 * time.Second, Multiplier: 2.0}
	h := http.Header{}
	h.Set("Retry-After", "30")
	d := BackoffDuration(cfg, 0, h)
	if d != 5*time.Second {
		t.Fatalf("expected capped 5s, got %s", d)
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultRetryConfig()
	if cfg.MaxRetries != 5 {
		t.Fatalf("expected 5 retries, got %d", cfg.MaxRetries)
	}
	if cfg.Multiplier != 2.0 {
		t.Fatalf("expected 2.0 multiplier, got %f", cfg.Multiplier)
	}
}
