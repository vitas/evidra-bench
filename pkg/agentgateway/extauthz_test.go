package agentgateway

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseExtAuthzDecisionAllow(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("testdata", "extauthz-allow.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	got, err := ParseExtAuthzDecision(data)
	if err != nil {
		t.Fatalf("ParseExtAuthzDecision() error = %v", err)
	}

	if got.Kind != KindPolicyDecision {
		t.Fatalf("kind = %q, want %q", got.Kind, KindPolicyDecision)
	}
	if got.Decision != DecisionAllow {
		t.Fatalf("decision = %q, want %q", got.Decision, DecisionAllow)
	}
	if got.PolicyID != "policy.safe" {
		t.Fatalf("policy_id = %q, want %q", got.PolicyID, "policy.safe")
	}
	if got.PolicyReason != "namespaced_patch" {
		t.Fatalf("policy_reason = %q, want %q", got.PolicyReason, "namespaced_patch")
	}
	if got.SessionID != "sess-123" {
		t.Fatalf("session_id = %q, want %q", got.SessionID, "sess-123")
	}
	if got.TraceID != "trace-allow-1" {
		t.Fatalf("trace_id = %q, want %q", got.TraceID, "trace-allow-1")
	}
}

func TestParseExtAuthzDecisionDeny(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("testdata", "extauthz-deny.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	got, err := ParseExtAuthzDecision(data)
	if err != nil {
		t.Fatalf("ParseExtAuthzDecision() error = %v", err)
	}

	if got.Decision != DecisionDeny {
		t.Fatalf("decision = %q, want %q", got.Decision, DecisionDeny)
	}
	if got.PolicyID != "policy.block" {
		t.Fatalf("policy_id = %q, want %q", got.PolicyID, "policy.block")
	}
	if got.PolicyReason != "cluster_scope_forbidden" {
		t.Fatalf("policy_reason = %q, want %q", got.PolicyReason, "cluster_scope_forbidden")
	}
}

func TestParseExtAuthzDecisionRejectsMissingDecision(t *testing.T) {
	t.Parallel()

	_, err := ParseExtAuthzDecision([]byte(`{"timestamp":"2026-03-18T12:35:00Z","attributes":{"mcp.session.id":"sess-123"}}`))
	if err == nil {
		t.Fatal("ParseExtAuthzDecision() error = nil, want non-nil")
	}
}
