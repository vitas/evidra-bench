package environment

import (
	"strings"
	"testing"
)

func TestSandboxRunArgsContract(t *testing.T) {
	t.Parallel()
	spec := SandboxSpec{
		RunID: "r1", Image: "img:1", Network: "net",
		AgentEnv: map[string]string{
			"INFRA_BENCH_SCENARIO":  "sid",
			"INFRA_BENCH_PROMPT":    "/mnt/evidra/agent/prompt.md",
			"INFRA_BENCH_WORKSPACE": "/workspace",
			"MY_KEY":                "v",
			"KUBECONFIG":            "/etc/passwd-please", // must stay sandbox-owned
			"PATH":                  "/broken",            // ditto
			"bad name":              "x",                  // rejected at injection
		},
	}
	args := sandboxRunArgs(spec, "vol", "name")
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"--read-only", "--cap-drop ALL", "--security-opt no-new-privileges",
		"--user 65534:65534",
		"--tmpfs /tmp:rw,size=64m",
		"--tmpfs /workspace:rw,size=256m", // the documented writable workspace
		"-e INFRA_BENCH_SCENARIO=sid",
		"-e INFRA_BENCH_PROMPT=/mnt/evidra/agent/prompt.md",
		"-e INFRA_BENCH_WORKSPACE=/workspace",
		"-e MY_KEY=v",
		"--entrypoint  img:1 sh -c sleep 2147483647",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args missing %q:\n%v", want, args)
		}
	}
	for _, ban := range []string{"KUBECONFIG=/etc", "PATH=/broken", "bad name"} {
		if strings.Contains(joined, ban) {
			t.Fatalf("reserved/junk env leaked into argv: %q", ban)
		}
	}
	if strings.Count(joined, "-e KUBECONFIG=") != 1 {
		t.Fatal("exactly one sandbox-owned KUBECONFIG injection expected")
	}
	// Deterministic order for the allowlisted env (sorted keys).
	spec2 := spec
	spec2.AgentEnv = map[string]string{"B": "2", "A": "1"}
	j2 := strings.Join(sandboxRunArgs(spec2, "vol", "name"), " ")
	if strings.Index(j2, "-e A=1") > strings.Index(j2, "-e B=2") {
		t.Fatalf("env must be sorted for reproducible argv: %v", j2)
	}
	// Custom workspace size honored.
	spec3 := spec
	spec3.WorkspaceSize = "1g"
	if !strings.Contains(strings.Join(sandboxRunArgs(spec3, "vol", "name"), " "), "/workspace:rw,size=1g") {
		t.Fatal("WorkspaceSize ignored")
	}
}
