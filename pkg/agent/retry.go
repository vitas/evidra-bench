package agent

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RetryConfig configures adaptive retry behavior.
type RetryConfig struct {
	MaxRetries     int           // max retry attempts (default: 3)
	InitialBackoff time.Duration // starting backoff (default: 2s)
	MaxBackoff     time.Duration // ceiling for backoff (default: 60s)
	Multiplier     float64       // backoff multiplier (default: 2.0)
}

// DefaultRetryConfig returns sensible defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     5,
		InitialBackoff: 2 * time.Second,
		MaxBackoff:     120 * time.Second,
		Multiplier:     2.0,
	}
}

// IsRetryable returns true if the HTTP status code is retryable.
func IsRetryable(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	default:
		return false
	}
}

// BackoffDuration calculates the next backoff duration.
// If the response includes a Retry-After header, uses that instead.
func BackoffDuration(cfg RetryConfig, attempt int, headers http.Header) time.Duration {
	// Check Retry-After header first
	if ra := headers.Get("Retry-After"); ra != "" {
		if seconds, err := strconv.ParseFloat(strings.TrimSpace(ra), 64); err == nil && seconds > 0 {
			d := time.Duration(seconds * float64(time.Second))
			if d > cfg.MaxBackoff {
				return cfg.MaxBackoff
			}
			return d
		}
		// Try parsing as HTTP date
		if t, err := http.ParseTime(ra); err == nil {
			d := time.Until(t)
			if d > 0 {
				if d > cfg.MaxBackoff {
					return cfg.MaxBackoff
				}
				return d
			}
		}
	}

	// Exponential backoff
	backoff := cfg.InitialBackoff
	for i := 0; i < attempt; i++ {
		backoff = time.Duration(float64(backoff) * cfg.Multiplier)
	}
	if backoff > cfg.MaxBackoff {
		backoff = cfg.MaxBackoff
	}
	return backoff
}

// SleepWithContext sleeps for the given duration, respecting context cancellation.
func SleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// RateLimitError wraps a rate limit response for logging.
type RateLimitError struct {
	StatusCode int
	Body       string
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited (HTTP %d), retry after %s", e.StatusCode, e.RetryAfter)
}
