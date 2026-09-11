package harness

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vitas/evidra-bench/pkg/audit"
	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/evaluation"
)

func TestSyntheticAgentBundleCopiesExecutablePath(t *testing.T) {
	t.Parallel()
	src := filepath.Join(t.TempDir(), "good.sh")
	if err := os.WriteFile(src, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	dir, err := syntheticAgentBundle(src)
	if err != nil {
		t.Fatalf("bundle: %v", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	data, err := os.ReadFile(filepath.Join(dir, "run"))
	if err != nil {
		t.Fatalf("read run: %v", err)
	}
	if !strings.Contains(string(data), "echo hi") {
		t.Fatalf("run must carry the script content, got %q", data)
	}
	st, err := os.Stat(filepath.Join(dir, "run"))
	if err != nil || st.Mode()&0o111 == 0 {
		t.Fatalf("run must be executable: %v %v", st, err)
	}
}

func TestSyntheticAgentBundleWrapsComplexCommand(t *testing.T) {
	t.Parallel()
	dir, err := syntheticAgentBundle(`bash -c "echo hi"`)
	if err != nil {
		t.Fatalf("bundle: %v", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	data, err := os.ReadFile(filepath.Join(dir, "run"))
	if err != nil {
		t.Fatalf("read run: %v", err)
	}
	if !strings.HasPrefix(string(data), "#!/bin/sh") || !strings.Contains(string(data), `exec bash -c "echo hi"`) {
		t.Fatalf("wrapped run unexpected: %q", data)
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
