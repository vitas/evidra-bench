package harness

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vitas/evidra-bench/pkg/adapter"
	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/environment"
	"github.com/vitas/evidra-bench/pkg/evaluation"
	"github.com/vitas/evidra-bench/pkg/scenario"
	"github.com/vitas/evidra-bench/pkg/verifier"
)

type fakeSandbox struct {
	calls  int
	spec   environment.SandboxSpec
	argv   []string
	result *environment.SandboxRun
	err    error
	avail  error
}

func (f *fakeSandbox) Available(context.Context) error { return f.avail }

func (f *fakeSandbox) Run(_ context.Context, spec environment.SandboxSpec, argv []string) (*environment.SandboxRun, error) {
	f.calls++
	f.spec, f.argv = spec, argv
	if f.err != nil {
		return nil, f.err
	}
	if f.result != nil {
		return f.result, nil
	}
	return &environment.SandboxRun{Stdout: "done", ExitCode: 0}, nil
}

func sandboxScenario(t *testing.T) (*scenario.Scenario, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "run"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "inputs", "notes.txt"), []byte("note"), 0o644); err != nil || os.MkdirAll(filepath.Join(dir, "inputs"), 0o755) != nil {
		_ = os.MkdirAll(filepath.Join(dir, "inputs"), 0o755)
		if err := os.WriteFile(filepath.Join(dir, "inputs", "notes.txt"), []byte("note"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s := &scenario.Scenario{ID: "sbx", Dir: dir, AgentInputs: []string{"inputs/notes.txt"}}
	return s, dir
}

func sandboxRequest(bundle string) RunRequest {
	return RunRequest{
		Config: config.Config{
			AgentImage: "agent:1", AgentBundleDir: bundle,
			SandboxMemory: "256m", SandboxCPUs: "0.5",
		},
		ClusterNetwork: "bench-network",
	}
}

func TestRunAgentSandboxedHappyPath(t *testing.T) {
	s, bundle := sandboxScenario(t)
	kc := filepath.Join(t.TempDir(), "kc")
	if err := os.WriteFile(kc, []byte("apiVersion: v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	fsbx := &fakeSandbox{result: &environment.SandboxRun{Stdout: "out", Stderr: "err", ExitCode: 0}}
	h := &Harness{deps: Deps{Sandbox: fsbx}}
	res, err := h.runAgentSandboxed(context.Background(), sandboxRequest(bundle), s, kc, "prompt text", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 || res.Stdout != "out" || res.Transcript != "outerr" {
		t.Fatalf("res = %+v", res)
	}
	if res.Metadata["sandbox"] != "1" || res.Metadata["sandbox_image"] != "agent:1" {
		t.Fatalf("metadata = %v", res.Metadata)
	}
	spec := fsbx.spec
	if spec.Image != "agent:1" || spec.Network != "bench-network" || spec.Memory != "256m" || spec.CPUs != "0.5" {
		t.Fatalf("spec = %+v", spec)
	}
	if spec.PromptContent != "prompt text" || spec.Kubeconfig != kc {
		t.Fatalf("prompt/kubeconfig not threaded: %+v", spec)
	}
	if spec.ExtraFiles["inputs/notes.txt"] != "note" {
		t.Fatalf("agent_inputs whitelist not staged: %v", spec.ExtraFiles)
	}
	if fsbx.argv[0] != "/mnt/evidra/agent/run" {
		t.Fatalf("argv = %v", fsbx.argv)
	}
}

func TestRunAgentSandboxedValidation(t *testing.T) {
	s, bundle := sandboxScenario(t)
	kc := filepath.Join(t.TempDir(), "kc")
	_ = os.WriteFile(kc, []byte("x"), 0o600)
	h := &Harness{deps: Deps{Sandbox: &fakeSandbox{}}}

	req := sandboxRequest(bundle)
	req.Config.AgentImage = ""
	if _, err := h.runAgentSandboxed(context.Background(), req, s, kc, "p", time.Minute); err == nil {
		t.Fatal("bundle without image must fail")
	}

	req = sandboxRequest("")
	if _, err := h.runAgentSandboxed(context.Background(), req, s, kc, "p", time.Minute); err == nil {
		t.Fatal("image without bundle must fail")
	}

	// Non-executable entrypoint.
	bad := t.TempDir()
	if err := os.WriteFile(filepath.Join(bad, "run"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := h.runAgentSandboxed(context.Background(), sandboxRequest(bad), s, kc, "p", time.Minute); err == nil {
		t.Fatal("non-executable ./run must fail")
	}
}

func TestRunAgentSandboxedUnavailableIsNotSilent(t *testing.T) {
	s, bundle := sandboxScenario(t)
	kc := filepath.Join(t.TempDir(), "kc")
	_ = os.WriteFile(kc, []byte("x"), 0o600)
	h := &Harness{deps: Deps{Sandbox: &fakeSandbox{avail: environment.ErrSandboxUnavailable}}}
	_, err := h.runAgentSandboxed(context.Background(), sandboxRequest(bundle), s, kc, "p", time.Minute)
	if !errors.Is(err, environment.ErrSandboxUnavailable) {
		t.Fatalf("err = %v, want sandbox unavailable", err)
	}
	kind, _ := classifyRunError(err, "agent_run")
	if kind != "sandbox_unavailable" {
		t.Fatalf("classified as %q, want sandbox_unavailable (INCOMPLETE)", kind)
	}
	// Missing cluster network also counts as unavailable, never fallback.
	req := sandboxRequest(bundle)
	req.ClusterNetwork = ""
	if _, err := h.runAgentSandboxed(context.Background(), req, s, kc, "p", time.Minute); !errors.Is(err, environment.ErrSandboxUnavailable) {
		t.Fatalf("no-network err = %v", err)
	}
}

func TestExecuteSingleAgentRoutesToSandbox(t *testing.T) {
	s, bundle := sandboxScenario(t)
	kc := filepath.Join(t.TempDir(), "kc")
	_ = os.WriteFile(kc, []byte("x"), 0o600)
	fsbx := &fakeSandbox{}
	h := &Harness{deps: Deps{Sandbox: fsbx}}
	res, err := h.executeSingleAgent(context.Background(), sandboxRequest(bundle), s, kc, "p", time.Minute, "")
	if err != nil {
		t.Fatal(err)
	}
	if fsbx.calls != 1 || res.Metadata["sandbox"] != "1" {
		t.Fatalf("sandbox routing failed: calls=%d res=%+v", fsbx.calls, res)
	}
}

// TestEvaluationRuntimeModes pins round-4 finding #2: modes, not a
// boolean. The old code called every sandbox-image-less run "unconfined",
// which falsely alarmed on mediated --model runs.
func TestEvaluationRuntimeModes(t *testing.T) {
	term := evaluation.Termination{Kind: evaluation.TerminationComplete}
	build := func(meta map[string]string) evaluation.CaseResult {
		return buildEvaluationCaseResult("c", "r1",
			&adapter.RunResult{ExitCode: 0, Metadata: meta},
			&verifier.VerifyResult{Passed: true, Checks: []verifier.CheckResult{{Verdict: verifier.VerdictPass}}},
			nil, "", time.Second, term, true, nil, nil, nil)
	}

	mediated := build(map[string]string{})
	if mediated.Runtime.Mode != evaluation.ModeMediated {
		t.Fatalf("in-process adapter must be mediated, got %q", mediated.Runtime.Mode)
	}
	if containsString(mediated.Safety.Gaps, evaluation.GapAgentUnconfined) {
		t.Fatalf("mediated runs are NOT unconfined and must not carry the gap: %v", mediated.Safety.Gaps)
	}

	sbx := build(map[string]string{"sandbox": "1", "sandbox_image": "agent:1"})
	if sbx.Runtime.Mode != evaluation.ModeSandboxed || sbx.Runtime.SandboxImage != "agent:1" {
		t.Fatalf("sandboxed run mislabeled: %+v", sbx.Runtime)
	}
	if containsString(sbx.Safety.Gaps, evaluation.GapAgentUnconfined) {
		t.Fatalf("sandboxed run must not carry unconfined gap: %v", sbx.Safety.Gaps)
	}

	ext := build(map[string]string{"agent_mode": "external-unconfined"})
	if ext.Runtime.Mode != evaluation.ModeExternalUnconfined {
		t.Fatalf("opt-out run mislabeled: %+v", ext.Runtime)
	}
	if !containsString(ext.Safety.Gaps, evaluation.GapAgentUnconfined) {
		t.Fatalf("only the genuine opt-out earns the gap: %v", ext.Safety.Gaps)
	}

	synth := build(map[string]string{"agent_mode": "external-sandboxed", "sandbox_image": "runner:1"})
	if synth.Runtime.Mode != evaluation.ModeSandboxed {
		t.Fatalf("synthetic-bundle runs are sandboxed: %+v", synth.Runtime)
	}

	remote := build(map[string]string{"agent_mode": "remote"})
	if remote.Runtime.Mode != evaluation.ModeRemoteUnattributed {
		t.Fatalf("a2a run mislabeled: %+v", remote.Runtime)
	}
	if containsString(remote.Safety.Gaps, evaluation.GapAgentUnconfined) {
		t.Fatalf("remote runs are not runner-privileged; no unconfined gap: %v", remote.Safety.Gaps)
	}
}
