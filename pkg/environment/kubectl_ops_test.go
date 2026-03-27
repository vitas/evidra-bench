package environment

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
)

// seqRunner records commands and returns preconfigured responses.
type seqRunner struct {
	responses []seqResponse
	calls     []string
	callIdx   int
}

type seqResponse struct {
	out []byte
	err error
}

func (s *seqRunner) Run(_ context.Context, cmd *exec.Cmd) ([]byte, error) {
	s.calls = append(s.calls, cmd.String())
	if s.callIdx < len(s.responses) {
		r := s.responses[s.callIdx]
		s.callIdx++
		return r.out, r.err
	}
	s.callIdx++
	return nil, nil
}

func TestHealthCheck_HealthyNode(t *testing.T) {
	t.Parallel()
	runner := &seqRunner{responses: []seqResponse{
		{out: []byte(`{"items":[{"metadata":{"name":"node1"},"status":{"conditions":[
			{"type":"Ready","status":"True"},
			{"type":"MemoryPressure","status":"False"},
			{"type":"DiskPressure","status":"False"},
			{"type":"PIDPressure","status":"False"}
		]}}]}`)},
		{out: []byte("")}, // no pending pods
	}}
	k := &kubectlOps{Runner: runner}
	if err := k.HealthCheck(context.Background(), "/tmp/kc"); err != nil {
		t.Fatalf("expected healthy, got: %v", err)
	}
}

func TestHealthCheck_MemoryPressure(t *testing.T) {
	t.Parallel()
	runner := &seqRunner{responses: []seqResponse{
		{out: []byte(`{"items":[{"metadata":{"name":"node1"},"status":{"conditions":[
			{"type":"Ready","status":"True"},
			{"type":"MemoryPressure","status":"True"}
		]}}]}`)},
	}}
	k := &kubectlOps{Runner: runner}
	err := k.HealthCheck(context.Background(), "/tmp/kc")
	if err == nil {
		t.Fatal("expected error for MemoryPressure")
	}
	if got := err.Error(); !contains(got, "MemoryPressure") {
		t.Fatalf("expected MemoryPressure in error, got: %s", got)
	}
}

func TestHealthCheck_NodeNotReady(t *testing.T) {
	t.Parallel()
	runner := &seqRunner{responses: []seqResponse{
		{out: []byte(`{"items":[{"metadata":{"name":"node1"},"status":{"conditions":[
			{"type":"Ready","status":"False"}
		]}}]}`)},
	}}
	k := &kubectlOps{Runner: runner}
	err := k.HealthCheck(context.Background(), "/tmp/kc")
	if err == nil {
		t.Fatal("expected error for not Ready")
	}
	if got := err.Error(); !contains(got, "not Ready") {
		t.Fatalf("expected 'not Ready' in error, got: %s", got)
	}
}

func TestHealthCheck_StuckPendingPod_NonSystem(t *testing.T) {
	t.Parallel()
	runner := &seqRunner{responses: []seqResponse{
		{out: []byte(`{"items":[{"metadata":{"name":"node1"},"status":{"conditions":[
			{"type":"Ready","status":"True"}
		]}}]}`)},
		{out: []byte("bench/stuck-pod ")}, // non-system namespace
	}}
	k := &kubectlOps{Runner: runner}
	err := k.HealthCheck(context.Background(), "/tmp/kc")
	if err == nil {
		t.Fatal("expected error for stuck pending pod")
	}
	if got := err.Error(); !contains(got, "bench/stuck-pod") {
		t.Fatalf("expected pod name in error, got: %s", got)
	}
}

func TestHealthCheck_SystemPendingPods_Ignored(t *testing.T) {
	t.Parallel()
	runner := &seqRunner{responses: []seqResponse{
		{out: []byte(`{"items":[{"metadata":{"name":"node1"},"status":{"conditions":[
			{"type":"Ready","status":"True"}
		]}}]}`)},
		{out: []byte("kube-system/coredns-abc local-path-storage/provisioner-xyz ")},
	}}
	k := &kubectlOps{Runner: runner}
	if err := k.HealthCheck(context.Background(), "/tmp/kc"); err != nil {
		t.Fatalf("system pods should be ignored, got: %v", err)
	}
}

func TestForceDeleteNamespace_NotExists(t *testing.T) {
	t.Parallel()
	runner := &seqRunner{responses: []seqResponse{
		{out: []byte("")}, // namespace doesn't exist
	}}
	k := &kubectlOps{Runner: runner}
	if err := k.ForceDeleteNamespace(context.Background(), "/tmp/kc", "bench"); err != nil {
		t.Fatalf("expected nil for non-existent namespace, got: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 call (get), got %d", len(runner.calls))
	}
}

func TestForceDeleteNamespace_ExistsAndDeletes(t *testing.T) {
	t.Parallel()
	runner := &seqRunner{responses: []seqResponse{
		{out: []byte("namespace/bench")}, // exists
		{out: []byte("")},                // delete succeeds
		{out: []byte("")},                // gone after poll
	}}
	k := &kubectlOps{Runner: runner}
	if err := k.ForceDeleteNamespace(context.Background(), "/tmp/kc", "bench"); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if len(runner.calls) < 3 {
		t.Fatalf("expected at least 3 calls (get, delete, check), got %d", len(runner.calls))
	}
}

func TestCreateNamespace_Success(t *testing.T) {
	t.Parallel()
	runner := &seqRunner{responses: []seqResponse{
		{out: []byte("namespace/bench created")},
	}}
	k := &kubectlOps{Runner: runner}
	if err := k.CreateNamespace(context.Background(), "/tmp/kc", "bench"); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestCreateNamespace_AlreadyExists(t *testing.T) {
	t.Parallel()
	runner := &seqRunner{responses: []seqResponse{
		{out: []byte("already exists"), err: fmt.Errorf("exit 1")},
	}}
	k := &kubectlOps{Runner: runner}
	if err := k.CreateNamespace(context.Background(), "/tmp/kc", "bench"); err != nil {
		t.Fatalf("already exists should be tolerated, got: %v", err)
	}
}

func TestCreateNamespace_OtherError(t *testing.T) {
	t.Parallel()
	runner := &seqRunner{responses: []seqResponse{
		{out: []byte("connection refused"), err: fmt.Errorf("exit 1")},
	}}
	k := &kubectlOps{Runner: runner}
	if err := k.CreateNamespace(context.Background(), "/tmp/kc", "bench"); err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestRunCanary_Success(t *testing.T) {
	t.Parallel()
	runner := &seqRunner{responses: []seqResponse{
		{out: []byte("")},   // delete leftover
		{out: []byte("ok")}, // canary run
	}}
	k := &kubectlOps{Runner: runner}
	if err := k.RunCanary(context.Background(), "/tmp/kc", "bench"); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestRunCanary_Failure(t *testing.T) {
	t.Parallel()
	runner := &seqRunner{responses: []seqResponse{
		{out: []byte("")}, // delete leftover
		{out: []byte("timeout"), err: fmt.Errorf("exit 1")}, // canary failed
	}}
	k := &kubectlOps{Runner: runner}
	if err := k.RunCanary(context.Background(), "/tmp/kc", "bench"); err == nil {
		t.Fatal("expected error for canary failure")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
