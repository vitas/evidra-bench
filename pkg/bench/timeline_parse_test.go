package bench

import (
	"encoding/json"
	"testing"
)

func makeToolCall(tool string, args map[string]string, result string) ToolCall {
	raw, _ := json.Marshal(args)
	return ToolCall{
		Tool:   tool,
		Args:   raw,
		Result: result,
	}
}

func TestParse_CommandFlow(t *testing.T) {
	t.Parallel()

	calls := []ToolCall{
		makeToolCall("run_command", map[string]string{"command": "kubectl get pods -n bench"}, "NAME  READY  STATUS\nweb   0/1    ErrImagePull"),
		makeToolCall("run_command", map[string]string{"command": "kubectl describe pod web-abc -n bench"}, "Events: Failed to pull image"),
		makeToolCall("run_command", map[string]string{"command": "kubectl patch deployment/web -n bench --type=json -p=[...]"}, "deployment.apps/web patched"),
		makeToolCall("run_command", map[string]string{"command": "kubectl rollout status deployment/web -n bench --timeout=60s"}, "deployment \"web\" successfully rolled out"),
	}

	tl := Parse(calls)

	if tl.TotalSteps != 4 {
		t.Fatalf("expected 4 steps, got %d", tl.TotalSteps)
	}

	expected := []struct {
		phase Phase
		tool  string
	}{
		{PhaseDiscover, "run_command"},
		{PhaseDiagnose, "run_command"},
		{PhaseAct, "run_command"},
		{PhaseVerify, "run_command"},
	}

	for i, want := range expected {
		got := tl.Steps[i]
		if got.Phase != want.phase {
			t.Errorf("step %d: phase = %q, want %q", i, got.Phase, want.phase)
		}
		if got.Tool != want.tool {
			t.Errorf("step %d: tool = %q, want %q", i, got.Tool, want.tool)
		}
	}

	if tl.MutationCount != 1 {
		t.Errorf("mutation_count = %d, want 1", tl.MutationCount)
	}
	if tl.DiagnosisDepth != 1 {
		t.Errorf("diagnosis_depth = %d, want 1", tl.DiagnosisDepth)
	}
	if tl.Steps[0].Namespace != "bench" {
		t.Errorf("step 0: namespace = %q, want %q", tl.Steps[0].Namespace, "bench")
	}
	if tl.Steps[0].Resource != "pods" {
		t.Errorf("step 0: resource = %q, want %q", tl.Steps[0].Resource, "pods")
	}
	if tl.Steps[2].Resource != "deployment/web" {
		t.Errorf("step 2: resource = %q, want %q", tl.Steps[2].Resource, "deployment/web")
	}
	if tl.Steps[0].Summary != "Listed pods in bench" {
		t.Errorf("step 0: summary = %q, want %q", tl.Steps[0].Summary, "Listed pods in bench")
	}
	if tl.Steps[2].Summary != "Patched deployment/web in bench" {
		t.Errorf("step 2: summary = %q, want %q", tl.Steps[2].Summary, "Patched deployment/web in bench")
	}
}

func TestParse_NoDecidePhaseForPlainCommands(t *testing.T) {
	t.Parallel()

	calls := []ToolCall{
		makeToolCall("run_command", map[string]string{"command": "kubectl get pods -n bench"}, "NAME  READY  STATUS\nweb   0/1    ErrImagePull"),
		makeToolCall("run_command", map[string]string{"command": "kubectl describe pod web-abc -n bench"}, "Events: ..."),
		makeToolCall("run_command", map[string]string{"command": "kubectl set image deployment/web nginx=nginx:stable -n bench"}, "deployment.apps/web image updated"),
		makeToolCall("run_command", map[string]string{"command": "kubectl rollout status deployment/web -n bench --timeout=60s"}, "deployment \"web\" successfully rolled out"),
	}

	tl := Parse(calls)

	if tl.TotalSteps != 4 {
		t.Fatalf("expected 4 steps, got %d", tl.TotalSteps)
	}

	expected := []Phase{PhaseDiscover, PhaseDiagnose, PhaseAct, PhaseVerify}
	for i, want := range expected {
		if tl.Steps[i].Phase != want {
			t.Errorf("step %d: phase = %q, want %q", i, tl.Steps[i].Phase, want)
		}
	}

	if tl.MutationCount != 1 {
		t.Errorf("mutation_count = %d, want 1", tl.MutationCount)
	}
	// No decide phase without an explicit decide signal.
	if count := tl.PhaseCount[PhaseDecide]; count != 0 {
		t.Errorf("decide phase count = %d, want 0", count)
	}
}

func TestParse_MultiMutation(t *testing.T) {
	t.Parallel()

	calls := []ToolCall{
		makeToolCall("run_command", map[string]string{"command": "kubectl get pods -n bench"}, "..."),
		makeToolCall("run_command", map[string]string{"command": "kubectl apply -f /tmp/fix1.yaml -n bench"}, "deployment.apps/web configured"),
		makeToolCall("run_command", map[string]string{"command": "kubectl apply -f /tmp/fix2.yaml -n bench"}, "service/web configured"),
		makeToolCall("run_command", map[string]string{"command": "kubectl get pods -n bench"}, "NAME  READY  STATUS\nweb   1/1    Running"),
	}

	tl := Parse(calls)

	if tl.TotalSteps != 4 {
		t.Fatalf("expected 4 steps, got %d", tl.TotalSteps)
	}

	expected := []Phase{PhaseDiscover, PhaseAct, PhaseAct, PhaseVerify}
	for i, want := range expected {
		if tl.Steps[i].Phase != want {
			t.Errorf("step %d: phase = %q, want %q", i, tl.Steps[i].Phase, want)
		}
	}

	if tl.MutationCount != 2 {
		t.Errorf("mutation_count = %d, want 2", tl.MutationCount)
	}
}

func TestParse_MCPResourceCreateOrUpdateMutation(t *testing.T) {
	t.Parallel()

	calls := []ToolCall{
		makeToolCall("resources_get", map[string]string{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"name":       "web",
			"namespace":  "bench",
		}, "apiVersion: apps/v1"),
		makeToolCall("resources_create_or_update", map[string]string{
			"resource": "apiVersion: v1\nkind: Service\nmetadata:\n  name: web\n  namespace: bench\n",
		}, "Service/web created"),
	}

	tl := Parse(calls)

	if tl.TotalSteps != 2 {
		t.Fatalf("total_steps = %d, want 2", tl.TotalSteps)
	}
	if tl.MutationCount != 1 {
		t.Fatalf("mutation_count = %d, want 1", tl.MutationCount)
	}
	step := tl.Steps[1]
	if step.Phase != PhaseAct {
		t.Fatalf("phase = %q, want %q", step.Phase, PhaseAct)
	}
	if step.Operation != "create_or_update" {
		t.Fatalf("operation = %q, want create_or_update", step.Operation)
	}
	if step.Resource != "Service/web" {
		t.Fatalf("resource = %q, want Service/web", step.Resource)
	}
	if step.Namespace != "bench" {
		t.Fatalf("namespace = %q, want bench", step.Namespace)
	}
}

func TestParse_MCPPodDeleteMutation(t *testing.T) {
	t.Parallel()

	calls := []ToolCall{
		makeToolCall("pods_delete", map[string]string{
			"name":      "web-abc",
			"namespace": "bench",
		}, "Pod deleted successfully"),
	}

	tl := Parse(calls)

	if tl.TotalSteps != 1 {
		t.Fatalf("total_steps = %d, want 1", tl.TotalSteps)
	}
	if tl.MutationCount != 1 {
		t.Fatalf("mutation_count = %d, want 1", tl.MutationCount)
	}
	step := tl.Steps[0]
	if step.Phase != PhaseAct {
		t.Fatalf("phase = %q, want %q", step.Phase, PhaseAct)
	}
	if step.Operation != "delete" {
		t.Fatalf("operation = %q, want delete", step.Operation)
	}
	if step.Resource != "Pod/web-abc" {
		t.Fatalf("resource = %q, want Pod/web-abc", step.Resource)
	}
}

func TestParse_EmptyCalls(t *testing.T) {
	t.Parallel()

	tl := Parse(nil)

	if tl.TotalSteps != 0 {
		t.Errorf("total_steps = %d, want 0", tl.TotalSteps)
	}
	if len(tl.Steps) != 0 {
		t.Errorf("steps length = %d, want 0", len(tl.Steps))
	}
	if tl.MutationCount != 0 {
		t.Errorf("mutation_count = %d, want 0", tl.MutationCount)
	}

	// Also test with empty slice.
	tl2 := Parse([]ToolCall{})
	if tl2.TotalSteps != 0 {
		t.Errorf("empty slice: total_steps = %d, want 0", tl2.TotalSteps)
	}
}

func TestParse_ReadOnlyOnly(t *testing.T) {
	t.Parallel()

	calls := []ToolCall{
		makeToolCall("run_command", map[string]string{"command": "kubectl get pods -n bench"}, "NAME  READY  STATUS\nweb   0/1    ErrImagePull"),
		makeToolCall("run_command", map[string]string{"command": "kubectl describe pod web-abc -n bench"}, "Events: ..."),
		makeToolCall("run_command", map[string]string{"command": "kubectl get deployment web -n bench -o yaml"}, "apiVersion: apps/v1\n..."),
		makeToolCall("run_command", map[string]string{"command": "kubectl logs web-abc -n bench"}, "error: container not running"),
	}

	tl := Parse(calls)

	if tl.TotalSteps != 4 {
		t.Fatalf("expected 4 steps, got %d", tl.TotalSteps)
	}

	expected := []Phase{PhaseDiscover, PhaseDiagnose, PhaseDiagnose, PhaseDiagnose}
	for i, want := range expected {
		if tl.Steps[i].Phase != want {
			t.Errorf("step %d: phase = %q, want %q", i, tl.Steps[i].Phase, want)
		}
	}

	if tl.MutationCount != 0 {
		t.Errorf("mutation_count = %d, want 0", tl.MutationCount)
	}
	if tl.DiagnosisDepth != 3 {
		t.Errorf("diagnosis_depth = %d, want 3", tl.DiagnosisDepth)
	}
	// No act or verify phases.
	if count := tl.PhaseCount[PhaseAct]; count != 0 {
		t.Errorf("act phase count = %d, want 0", count)
	}
	if count := tl.PhaseCount[PhaseVerify]; count != 0 {
		t.Errorf("verify phase count = %d, want 0", count)
	}
}

func TestParse_IgnoresLegacyPrescribeAlias(t *testing.T) {
	t.Parallel()

	calls := []ToolCall{
		makeToolCall("run_command", map[string]string{"command": "kubectl get pods -n bench"}, "NAME  READY  STATUS\nweb   0/1    ErrImagePull"),
		makeToolCall("evidra_prescribe", map[string]string{"operation": "patch", "tool": "kubectl"}, `{"ok":true,"prescription_id":"abc123"}`),
		makeToolCall("run_command", map[string]string{"command": "kubectl patch deployment/web -n bench --type=json -p=[...]"}, "deployment.apps/web patched"),
	}

	tl := Parse(calls)

	if tl.TotalSteps != 2 {
		t.Fatalf("expected 2 steps after ignoring legacy prescribe alias, got %d", tl.TotalSteps)
	}

	if count := tl.PhaseCount[PhaseDecide]; count != 0 {
		t.Fatalf("decide phase count = %d, want 0 when only legacy alias is present", count)
	}

	expected := []Phase{PhaseDiscover, PhaseAct}
	for i, want := range expected {
		if tl.Steps[i].Phase != want {
			t.Errorf("step %d: phase = %q, want %q", i, tl.Steps[i].Phase, want)
		}
	}
}
