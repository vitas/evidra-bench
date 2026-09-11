package harness

import (
	"testing"
	"time"
)

func TestIdentityReadyTimeout(t *testing.T) {
	if got := identityReadyTimeout(); got != 10*time.Minute {
		t.Fatalf("default = %v", got)
	}
	t.Setenv("EVIDRA_IDENTITY_READY_TIMEOUT", "45s")
	if got := identityReadyTimeout(); got != 45*time.Second {
		t.Fatalf("override = %v", got)
	}
	t.Setenv("EVIDRA_IDENTITY_READY_TIMEOUT", "nonsense")
	if got := identityReadyTimeout(); got != 10*time.Minute {
		t.Fatalf("invalid input must fall back to default, got %v", got)
	}
}
