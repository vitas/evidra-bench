package harness

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vitas/evidra-bench/pkg/adapter"
	"github.com/vitas/evidra-bench/pkg/audit"
	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/evaluation"
	"github.com/vitas/evidra-bench/pkg/scenario"
	"github.com/vitas/evidra-bench/pkg/verifier"
)

func TestSyntheticAgentBundleCopiesExecutablePath(t *testing.T) {
	t.Parallel()
	src := filepath.Join(t.TempDir(), "good.sh")
	if err := os.WriteFile(src, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	dir, err := syntheticAgentBundle(src, nil, false)
	if err != nil {
		t.Fatalf("bundle: %v", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	data, err := os.ReadFile(filepath.Join(dir, "bin", "good.sh"))
	if err != nil {
		t.Fatalf("exe must be copied under bin/: %v", err)
	}
	if !strings.Contains(string(data), "echo hi") {
		t.Fatalf("copy lost content: %q", data)
	}
	run, err := os.ReadFile(filepath.Join(dir, "run"))
	if err != nil || len(run) == 0 {
		t.Fatalf("read run: %v", err)
	}
	if !strings.Contains(string(run), "/mnt/evidra/agent/bin/'good.sh'") {
		t.Fatalf("run wrapper must exec the copied exe: %q", run)
	}
}

// The round-4 blocker case: `--agent "./my-agent --config agent.yaml"`
// must really run the binary with its arguments AND find the config file.
func TestSyntheticAgentBundleWithArgumentsAndInputs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	exe := filepath.Join(root, "my-agent")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfgFile := filepath.Join(root, "agent.yaml")
	if err := os.WriteFile(cfgFile, []byte("mode: demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir, err := syntheticAgentBundle(exe+" --config agent.yaml", []string{cfgFile}, false)
	if err != nil {
		t.Fatalf("bundle: %v", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	st, err := os.Stat(filepath.Join(dir, "bin", "my-agent"))
	if err != nil || st.Mode()&0o111 == 0 {
		t.Fatalf("exe not staged executable: %v %v", st, err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "agent.yaml"))
	if err != nil || !strings.Contains(string(data), "mode: demo") {
		t.Fatalf("input not staged: %q %v", data, err)
	}
	run, _ := os.ReadFile(filepath.Join(dir, "run"))
	if !strings.Contains(string(run), "'--config' 'agent.yaml'") {
		t.Fatalf("wrapper lost quoted arguments: %q", run)
	}
	if !strings.Contains(string(run), `cd "$(dirname "$0")"`) {
		t.Fatalf("wrapper must cd into the bundle so config paths resolve: %q", run)
	}
}

func TestSyntheticAgentBundleResolvesPathExecutable(t *testing.T) {
	// t.Setenv forbids t.Parallel.
	bin := t.TempDir()
	fake := filepath.Join(bin, "kubectl-ai")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	dir, err := syntheticAgentBundle("kubectl-ai --para", nil, false)
	if err != nil {
		t.Fatalf("bare PATH name must resolve and be copied: %v", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if _, err := os.Stat(filepath.Join(dir, "bin", "kubectl-ai")); err != nil {
		t.Fatalf("PATH-resolved exe missing: %v", err)
	}
}

func TestSyntheticAgentBundleRefusesUnresolvableCommand(t *testing.T) {
	t.Parallel()
	_, err := syntheticAgentBundle("definitely-not-installed-binary-xyz --flag", nil, false)
	if err == nil {
		t.Fatal("must refuse: first token is not a local executable and no custom image was stated")
	}
	for _, want := range []string{"--agent-input", "--agent-image", "--agent-bundle", "--agent-unconfined"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error must teach every way out (missing %s): %v", want, err)
		}
	}
	// With an explicit custom image the command passes through verbatim.
	dir, err := syntheticAgentBundle("definitely-not-installed-binary-xyz --flag", nil, true)
	if err != nil {
		t.Fatalf("custom image must allow pass-through: %v", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	run, _ := os.ReadFile(filepath.Join(dir, "run"))
	if !strings.Contains(string(run), "exec definitely-not-installed-binary-xyz --flag") {
		t.Fatalf("pass-through wrapper wrong: %q", run)
	}
}

func TestSyntheticAgentBundleInputCollision(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	exe := filepath.Join(root, "tool")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	dup1 := filepath.Join(root, "cfg.yaml")
	dup2 := filepath.Join(t.TempDir(), "cfg.yaml")
	if err := os.WriteFile(dup1, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dup2, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := syntheticAgentBundle(exe, []string{dup1, dup2}, false); err == nil {
		t.Fatal("same-basename inputs must be a loud error, not silent last-wins")
	}
}

func TestSplitAgentCommandQuotes(t *testing.T) {
	t.Parallel()
	got, err := splitAgentCommand("./a --msg \"two words\" --x 'sin-gle' --esc\\ t")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"./a", "--msg", "two words", "--x", "sin-gle", "--esc t"}
	if len(got) != len(want) {
		t.Fatalf("fields = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("field %d = %q want %q", i, got[i], want[i])
		}
	}
	if _, err := splitAgentCommand(`./a "unterminated`); err == nil {
		t.Fatal("unterminated quote must error")
	}
}

func TestBuildAgentEnvContract(t *testing.T) {
	// t.Setenv forbids t.Parallel.
	t.Setenv("MY_AGENT_KEY", "secret-value")
	t.Setenv("ABSENT_NAME", "")
	_ = os.Unsetenv("ABSENT_NAME")
	req := RunRequest{Config: config.Default()}
	req.Config.AgentEnv = []string{"MY_AGENT_KEY", "ABSENT_NAME", "KUBECONFIG", "PATH", "bad name"}
	env := buildAgentEnv(&scenario.Scenario{ID: "sid"}, req)
	if env["INFRA_BENCH_SCENARIO"] != "sid" ||
		env["INFRA_BENCH_PROMPT"] != "/mnt/evidra/agent/prompt.md" ||
		env["INFRA_BENCH_WORKSPACE"] != "/workspace" {
		t.Fatalf("documented contract missing: %v", env)
	}
	if env["MY_AGENT_KEY"] != "secret-value" {
		t.Fatalf("allowlisted var missing: %v", env)
	}
	if _, ok := env["ABSENT_NAME"]; ok {
		t.Fatal("absent vars must not appear as empty")
	}
	if _, ok := env["bad name"]; ok {
		t.Fatal("junk name must not leak in (sandbox refuses it too)")
	}
	// KUBECONFIG/PATH are owned by the sandbox; never overridable.
	if env["KUBECONFIG"] != "" || env["PATH"] != "" {
		t.Fatalf("reserved names leaked: %v", env)
	}
}

func TestSyntheticAgentBundleWrapsComplexCommand(t *testing.T) {
	t.Parallel()
	dir, err := syntheticAgentBundle(`bash -c "echo hi"`, nil, false)
	if err != nil {
		t.Fatalf("bundle: %v", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	data, err := os.ReadFile(filepath.Join(dir, "run"))
	if err != nil {
		t.Fatalf("read run: %v", err)
	}
	// bash resolves on PATH and is copied; args are preserved quoted.
	if !strings.HasPrefix(string(data), "#!/bin/sh") || !strings.Contains(string(data), "'-c' 'echo hi'") {
		t.Fatalf("wrapped run unexpected: %q", data)
	}
	if st, err := os.Stat(filepath.Join(dir, "bin", "bash")); err != nil || st.Size() == 0 {
		t.Skipf("bash not copyable on this platform (resolved=%v): %v", st, err)
	}
}

func TestResolveAgentSandboxImagePrecedence(t *testing.T) {
	// t.Setenv forbids t.Parallel.
	h := New(Deps{})
	ctx := context.Background()

	// Explicit flag wins.
	req := RunRequest{Config: config.Default()}
	req.Config.AgentImage = "explicit:1"
	if got, err := h.resolveAgentSandboxImage(ctx, req); err != nil || got != "explicit:1" {
		t.Fatalf("explicit image: %q %v", got, err)
	}
	// Env override next.
	t.Setenv("EVIDRA_AGENT_SANDBOX_IMAGE", "env:2")
	req.Config.AgentImage = ""
	if got, err := h.resolveAgentSandboxImage(ctx, req); err != nil || got != "env:2" {
		t.Fatalf("env image: %q %v", got, err)
	}
	// Without either, discovery must fail cleanly outside a container.
	t.Setenv("EVIDRA_AGENT_SANDBOX_IMAGE", "")
	t.Setenv("HOSTNAME", "")
	req2 := RunRequest{Config: config.Default()}
	if _, err := h.resolveAgentSandboxImage(ctx, req2); err == nil {
		t.Fatal("undiscoverable image must error (never a silent unconfined fallback)")
	}
}

// An unconfined EXTERNAL agent under an authority profile must land the
// case on INCOMPLETE even when every check passes (release review #2).
func TestExternalUnconfinedAgentGradesIncomplete(t *testing.T) {
	t.Parallel()
	prof := testAuthorityProfile("kube-system/services/web")
	in := buildEngineInput(prof, &AuditWindowInfo{Result: &audit.Result{
		Window: auditWindow(nil), Coverage: audit.CoverageComplete,
	}}, &SnapshotInfo{Coverage: evaluation.CoverageComplete}, false, false, true, true)
	ev := evaluation.AuthoritativeVerdict(*in)
	if ev.Verdict != evaluation.VerdictIncomplete || ev.Eligible {
		t.Fatalf("unconfined external agent must be INCOMPLETE: %+v", ev)
	}
	if got := engineIncompletionReason(*in); got != "agent_unconfined" {
		t.Fatalf("reason = %q", got)
	}
	// Confined (the default) keeps the clean PASS path.
	in.AgentUnconfined = false
	if ev := evaluation.AuthoritativeVerdict(*in); ev.Verdict != evaluation.VerdictPass {
		t.Fatalf("confined clean run must PASS: %+v", ev)
	}
}

// Round-5 finding #1: the custom-image pass-through must honor the same
// --agent-input contract as the local-executable branch — the wrapper has
// to cd into the bundle, or staged files sit in a directory the command
// never sees (the image WORKDIR wins otherwise).
func TestSyntheticAgentBundleCustomImageWrapperCdAndInputs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cfgFile := filepath.Join(root, "agent.yaml")
	if err := os.WriteFile(cfgFile, []byte("mode: custom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir, err := syntheticAgentBundle("my-agent --config agent.yaml", []string{cfgFile}, true)
	if err != nil {
		t.Fatalf("custom image bundle: %v", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	run, err := os.ReadFile(filepath.Join(dir, "run"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(run), `cd "$(dirname "$0")"`) {
		t.Fatalf("custom-image wrapper must cd into the bundle dir: %q", run)
	}
	if !strings.Contains(string(run), "exec my-agent --config agent.yaml") {
		t.Fatalf("command must pass through verbatim: %q", run)
	}
	data, err := os.ReadFile(filepath.Join(dir, "agent.yaml"))
	if err != nil || !strings.Contains(string(data), "mode: custom") {
		t.Fatalf("inputs must stage into custom-image bundles too: %q %v", data, err)
	}
}

// Round-5 finding #2: a failed agent result with NO observed metadata
// must carry the PLANNED mode, never mediated-by-default.
func TestFailedRunKeepsPlannedMode(t *testing.T) {
	term := evaluation.Termination{Kind: evaluation.TerminationComplete}
	nAgent := failedAgentResult(nil, 1) // exactly what the failure path normalizes to
	crp := config.Default()

	sandboxed := buildEvaluationCaseResultPlanned("c", plannedAgentMode(RunRequest{Config: func() config.Config {
		cp := crp
		cp.AgentCommand = "/somewhere/agent.sh"
		return cp
	}()}), "r1", nAgent,
		&verifier.VerifyResult{Checks: []verifier.CheckResult{{Verdict: verifier.VerdictFail}}},
		nil, "", time.Second, term, false, nil, nil, nil)
	if sandboxed.Runtime.Mode != evaluation.ModeSandboxed {
		t.Fatalf("sandbox-unavailable run mislabeled %q", sandboxed.Runtime.Mode)
	}
	if containsString(sandboxed.Safety.Gaps, evaluation.GapAgentUnconfined) {
		t.Fatal("planned-sandboxed is not unconfined; no gap")
	}

	unconf := buildEvaluationCaseResultPlanned("c", plannedAgentMode(RunRequest{Config: func() config.Config {
		cp := crp
		cp.AgentCommand = "/somewhere/agent.sh"
		cp.AgentUnconfined = true
		return cp
	}()}), "r2", nAgent,
		&verifier.VerifyResult{Passed: true, Checks: []verifier.CheckResult{{Verdict: verifier.VerdictPass}}},
		nil, "", time.Second, term, true, nil, nil, nil)
	if unconf.Runtime.Mode != evaluation.ModeExternalUnconfined {
		t.Fatalf("planned opt-out mislabeled %q", unconf.Runtime.Mode)
	}
	if !containsString(unconf.Safety.Gaps, evaluation.GapAgentUnconfined) {
		t.Fatal("planned external_unconfined keeps the gap on failed results")
	}
	// Observed metadata beats the plan.
	observed := buildEvaluationCaseResultPlanned("c", evaluation.ModeSandboxed, "r3",
		&adapter.RunResult{ExitCode: 0, Metadata: map[string]string{"agent_mode": "remote"}},
		&verifier.VerifyResult{Passed: true, Checks: []verifier.CheckResult{{Verdict: verifier.VerdictPass}}},
		nil, "", time.Second, term, false, nil, nil, nil)
	if observed.Runtime.Mode != evaluation.ModeRemoteUnattributed {
		t.Fatalf("observed remote must beat planned sandboxed: %q", observed.Runtime.Mode)
	}

	// The plain-adapter plan stays mediated (no regression for --model).
	if m := plannedAgentMode(RunRequest{Config: crp}); m != evaluation.ModeMediated {
		t.Fatalf("default plan = %q, want mediated", m)
	}
}
