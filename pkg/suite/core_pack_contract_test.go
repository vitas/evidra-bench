package suite

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vitas/evidra-bench/pkg/scenario"
)

// TestKubernetesCoreV1AdmissionContract pins, for every case the
// kubernetes-core@1 manifest names, the STATIC half of the admission
// contract from the implementation plan:
//
//   - loads through pkg/scenario at all,
//   - declares a non-empty authority profile with on_denied: unsafe,
//     at least one agent rule, and at least one protected object,
//   - verifies exclusively through assert-v2 checks (no
//     command-succeeds survivors), with an executable fixture that
//     speaks the protocol (emits "assertions"),
//   - has scripted safe AND unsafe control arms in the fixture agent —
//     without both, the exact-verdict matrix cannot even be run.
//
// The DYNAMIC half (the 3-run exact matrix per provider) is proven by
// make core-contract-kind / core-contract-k3d, not here.
func TestKubernetesCoreV1AdmissionContract(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(filepath.Join(root, "suites", "kubernetes-core-v1.yaml"), root)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	scriptPath := filepath.Join(root, "tests", "fixtures", "scripted-agent", "kubernetes-core.sh")
	agentScript := readFile(t, scriptPath)
	safeBody := functionBody(t, agentScript, "safe_repair")
	unsafeBody := functionBody(t, agentScript, "unsafe_attempt")

	for _, entry := range loaded.Scenarios {
		id := entry.ID
		t.Run(id, func(t *testing.T) {
			s, err := scenario.Load(filepath.Join(root, "scenarios", "kubernetes", id))
			if err != nil {
				t.Fatalf("scenario load: %v", err)
			}
			p := s.AuthorityProfile
			if p == nil {
				t.Fatal("no authority_profile")
			}
			if len(p.Agent.Rules) == 0 {
				t.Fatal("authority_profile has no agent rules")
			}
			if len(p.Protected) == 0 {
				t.Fatal("authority_profile protects nothing")
			}
			if p.Agent.OnDenied != scenario.OnDeniedUnsafe {
				t.Fatalf("on_denied = %q, want %q", p.Agent.OnDenied, scenario.OnDeniedUnsafe)
			}

			assertV2 := 0
			for _, c := range s.Checks {
				switch c.Type {
				case "command-succeeds":
					t.Fatalf("legacy command-succeeds check survives: %s", c.Name)
				case scenario.CheckTypeAssertV2:
					assertV2++
					if c.Condition == "" {
						t.Fatalf("assert-v2 %q has no condition", c.Name)
					}
					cond := c.Condition
					if !filepath.IsAbs(cond) {
						cond = filepath.Join(root, "scenarios", "kubernetes", id, cond)
					}
					body := readFile(t, cond)
					if !strings.Contains(body, `"assertions"`) {
						t.Fatalf("assert-v2 fixture %s does not emit the protocol", c.Condition)
					}
					st, err := os.Stat(cond)
					if err != nil {
						t.Fatal(err)
					}
					if st.Mode().Perm()&0o111 == 0 {
						t.Fatalf("%s is not executable", c.Condition)
					}
				}
			}
			if assertV2 == 0 {
				t.Fatal("no assert-v2 check")
			}
			arm := "\n    " + id + ")\n"
			if !strings.Contains(safeBody, arm) {
				t.Fatalf("kubernetes-core.sh has no safe_repair arm for %s", id)
			}
			if !strings.Contains(unsafeBody, arm) {
				t.Fatalf("kubernetes-core.sh has no unsafe_attempt arm for %s", id)
			}
		})
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// functionBody returns the text of a top-level bash function of the form
// `name() { ... }` terminated by a line that is just `}`.
func functionBody(t *testing.T, script, name string) string {
	t.Helper()
	start := strings.Index(script, "\n"+name+"() {\n")
	if start < 0 {
		t.Fatalf("function %s not found", name)
	}
	rest := script[start:]
	end := strings.Index(rest, "\n}\n")
	if end < 0 {
		t.Fatalf("function %s unterminated", name)
	}
	return rest[:end]
}
