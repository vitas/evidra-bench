package agentgateway

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseOTELRecord(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("testdata", "mcp-call.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	got, err := ParseOTELRecord(data)
	if err != nil {
		t.Fatalf("ParseOTELRecord() error = %v", err)
	}

	if got.Kind != KindGatewayCall {
		t.Fatalf("kind = %q, want %q", got.Kind, KindGatewayCall)
	}
	if got.Method != "tools/call" {
		t.Fatalf("method = %q, want %q", got.Method, "tools/call")
	}
	if got.Tool != "kubectl_apply" {
		t.Fatalf("tool = %q, want %q", got.Tool, "kubectl_apply")
	}
	if got.Target != "cluster-a" {
		t.Fatalf("target = %q, want %q", got.Target, "cluster-a")
	}
	if got.SessionID != "sess-123" {
		t.Fatalf("session_id = %q, want %q", got.SessionID, "sess-123")
	}
	if got.Status != 200 {
		t.Fatalf("status = %d, want %d", got.Status, 200)
	}
	if got.Reason != "upstream" {
		t.Fatalf("reason = %q, want %q", got.Reason, "upstream")
	}
}

func TestParseOTELRecordRejectsMissingMethod(t *testing.T) {
	t.Parallel()

	_, err := ParseOTELRecord([]byte(`{"timestamp":"2026-03-18T12:34:56Z","attributes":{"mcp.target":"cluster-a"}}`))
	if err == nil {
		t.Fatal("ParseOTELRecord() error = nil, want non-nil")
	}
}
