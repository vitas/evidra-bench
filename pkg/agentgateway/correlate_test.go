package agentgateway

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCorrelateClusterEffects(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("testdata", "k8s-audit.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	base := time.Date(2026, time.March, 18, 12, 35, 2, 0, time.UTC)
	events := []Event{
		{
			Kind:      KindGatewayCall,
			Timestamp: base,
			SessionID: "sess-123",
			Target:    "cluster-a",
			Namespace: "bench",
			Resource:  "deployment/web",
			Method:    "tools/call",
			Tool:      "kubectl_apply",
			Status:    200,
		},
	}

	effects, unmatched, err := CorrelateClusterEffects(events, data)
	if err != nil {
		t.Fatalf("CorrelateClusterEffects() error = %v", err)
	}

	if len(effects) != 1 {
		t.Fatalf("len(effects) = %d, want 1", len(effects))
	}
	if len(unmatched) != 1 {
		t.Fatalf("len(unmatched) = %d, want 1", len(unmatched))
	}

	got := effects[0]
	if got.Kind != KindClusterEffect {
		t.Fatalf("kind = %q, want %q", got.Kind, KindClusterEffect)
	}
	if got.Namespace != "bench" {
		t.Fatalf("namespace = %q, want %q", got.Namespace, "bench")
	}
	if got.Verb != "patch" {
		t.Fatalf("verb = %q, want %q", got.Verb, "patch")
	}
	if got.ResourceKind != "deployment" {
		t.Fatalf("resource_kind = %q, want %q", got.ResourceKind, "deployment")
	}
	if got.ResourceName != "web" {
		t.Fatalf("resource_name = %q, want %q", got.ResourceName, "web")
	}
	if got.ResultCode != 200 {
		t.Fatalf("result_code = %d, want %d", got.ResultCode, 200)
	}
	if got.SessionID != "sess-123" {
		t.Fatalf("session_id = %q, want %q", got.SessionID, "sess-123")
	}
	if got.Target != "cluster-a" {
		t.Fatalf("target = %q, want %q", got.Target, "cluster-a")
	}
}
