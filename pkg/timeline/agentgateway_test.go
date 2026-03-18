package timeline

import (
	"testing"
	"time"

	"samebits.com/evidra-infra-bench/pkg/agentgateway"
)

func TestParseAgentGateway(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.March, 18, 12, 35, 0, 0, time.UTC)
	events := []agentgateway.Event{
		{
			Kind:         agentgateway.KindClusterEffect,
			Timestamp:    base.Add(3 * time.Second),
			Namespace:    "bench",
			Verb:         "patch",
			ResourceKind: "deployment",
			ResourceName: "web",
		},
		{
			Kind:         agentgateway.KindPolicyDecision,
			Timestamp:    base,
			SessionID:    "sess-123",
			Decision:     agentgateway.DecisionDeny,
			PolicyID:     "policy.block",
			PolicyReason: "cluster_scope_forbidden",
		},
		{
			Kind:      agentgateway.KindGatewayCall,
			Timestamp: base.Add(2 * time.Second),
			SessionID: "sess-123",
			Target:    "cluster-a",
			Method:    "tools/call",
			Tool:      "kubectl_apply",
			Status:    200,
		},
		{
			Kind:         agentgateway.KindPolicyDecision,
			Timestamp:    base.Add(1 * time.Second),
			SessionID:    "sess-123",
			Decision:     agentgateway.DecisionAllow,
			PolicyID:     "policy.safe",
			PolicyReason: "namespaced_patch",
		},
	}

	tl := ParseAgentGateway(events)

	if tl.TotalSteps != 4 {
		t.Fatalf("total_steps = %d, want 4", tl.TotalSteps)
	}
	if got := tl.Steps[0].Kind; got != string(agentgateway.KindPolicyDecision) {
		t.Fatalf("step 0 kind = %q, want %q", got, agentgateway.KindPolicyDecision)
	}
	if got := tl.Steps[1].Kind; got != string(agentgateway.KindPolicyDecision) {
		t.Fatalf("step 1 kind = %q, want %q", got, agentgateway.KindPolicyDecision)
	}
	if got := tl.Steps[2].Kind; got != string(agentgateway.KindGatewayCall) {
		t.Fatalf("step 2 kind = %q, want %q", got, agentgateway.KindGatewayCall)
	}
	if got := tl.Steps[3].Kind; got != string(agentgateway.KindClusterEffect) {
		t.Fatalf("step 3 kind = %q, want %q", got, agentgateway.KindClusterEffect)
	}

	expectedPhases := []Phase{PhaseDecide, PhaseDecide, PhaseAct, PhaseVerify}
	for i, want := range expectedPhases {
		if got := tl.Steps[i].Phase; got != want {
			t.Fatalf("step %d phase = %q, want %q", i, got, want)
		}
	}

	if tl.MutationCount != 1 {
		t.Fatalf("mutation_count = %d, want 1", tl.MutationCount)
	}
	if tl.Steps[2].Target != "cluster-a" {
		t.Fatalf("step 2 target = %q, want %q", tl.Steps[2].Target, "cluster-a")
	}
	if tl.Steps[3].Namespace != "bench" {
		t.Fatalf("step 3 namespace = %q, want %q", tl.Steps[3].Namespace, "bench")
	}
	if tl.Steps[3].Summary != "Patched deployment/web in bench" {
		t.Fatalf("step 3 summary = %q, want %q", tl.Steps[3].Summary, "Patched deployment/web in bench")
	}
}
