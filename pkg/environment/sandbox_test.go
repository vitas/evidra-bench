package environment

import (
	"archive/tar"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeDocker installs a logging docker stub on PATH for the test duration.
// Behavior switches are files in dir: fail-start, exec-code.
func fakeDocker(t *testing.T, dir string) (logPath string, calls *string) {
	t.Helper()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath = filepath.Join(dir, "docker.calls")
	script := `#!/bin/sh
echo "docker $*" >> "` + logPath + `"
case "$1" in
  version) echo "27.0.0" ;;
  volume)
    if [ "$2" = create ]; then echo "$3"; fi ;;
  run)
    for a in "$@"; do [ "$a" = -d ] && { [ -f ` + dir + `/fail-start ] && exit 1; echo abc123; exit 0; }; done
    exit 0 ;;
  exec)
    code=0; [ -f ` + dir + `/exec-code ] && code=$(cat ` + dir + `/exec-code)
    echo "agent stdout line"
    exit "$code" ;;
esac
exit 0
`
	stub := filepath.Join(bin, "docker")
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	return logPath, nil
}

func readCalls(t *testing.T, logPath string) string {
	t.Helper()
	b, err := os.ReadFile(logPath)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func testSpec(t *testing.T) SandboxSpec {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "bundle"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bundle", "run"), []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	kc := filepath.Join(dir, "kc")
	if err := os.WriteFile(kc, []byte("kubeconfig-content"), 0o600); err != nil {
		t.Fatal(err)
	}
	return SandboxSpec{
		RunID: "run42", Image: "agent:1", BundleDir: filepath.Join(dir, "bundle"),
		PromptContent: "do the thing", Kubeconfig: kc,
		ExtraFiles: map[string]string{"notes.txt": "whitelisted input"},
		Network:    "bench-network", Memory: "512m", CPUs: "1.0",
		Timeout: 30 * time.Second,
	}
}

func TestSandboxAgentEnvChannel(t *testing.T) {
	dir := t.TempDir()
	logPath, _ := fakeDocker(t, dir)
	s := &DockerSandbox{}
	spec := testSpec(t)
	spec.AgentEnv = map[string]string{
		"INFRA_BENCH_SCENARIO": "broken-deployment",
		"bad-key;rm -rf /":     "nope",
		"PATH":                 "/evil",
	}
	if _, err := s.Run(context.Background(), spec, []string{"/mnt/evidra/agent/run"}); err != nil {
		t.Fatal(err)
	}
	start := ""
	for _, l := range strings.Split(readCalls(t, logPath), "\n") {
		if strings.HasPrefix(l, "docker run -d") {
			start = l
		}
	}
	if !strings.Contains(start, "-e INFRA_BENCH_SCENARIO=broken-deployment") {
		t.Fatalf("named env must pass: %s", start)
	}
	if !strings.Contains(start, "-e HOME=/tmp -e INFRA_BENCH_SCENARIO") && strings.Count(start, "HOME=") != 1 {
		t.Fatalf("HOME must appear exactly once (reserved wins): %s", start)
	}
	for _, banned := range []string{"bad-key", "-e PATH=", "-e HOME=/evil"} {
		if strings.Contains(start, banned) {
			t.Fatalf("%q must never reach the sandbox argv: %s", banned, start)
		}
	}
}

func TestSandboxHardeningProfile(t *testing.T) {
	dir := t.TempDir()
	logPath, _ := fakeDocker(t, dir)
	s := &DockerSandbox{}
	spec := testSpec(t)
	if _, err := s.Run(context.Background(), spec, []string{"/mnt/evidra/agent/run"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	calls := readCalls(t, logPath)
	var startLine string
	for _, l := range strings.Split(calls, "\n") {
		if strings.HasPrefix(l, "docker run -d") {
			startLine = l
		}
	}
	if startLine == "" {
		t.Fatalf("no detached run call in:\n%s", calls)
	}
	for _, want := range []string{
		"--read-only", "--cap-drop ALL", "--security-opt no-new-privileges",
		"--user 65534:65534", "--network bench-network", "--tmpfs",
		"--memory 512m", "--cpus 1.0",
		"-v evidra-agent-run42:/mnt/evidra:ro",
		"--entrypoint",
		"-e KUBECONFIG=/mnt/evidra/run/agent.kubeconfig",
	} {
		if !strings.Contains(startLine, want) {
			t.Errorf("hardening flag %q missing from:\n%s", want, startLine)
		}
	}
	if strings.Contains(startLine, "-v /") {
		t.Errorf("sandbox must never bind-mount host paths:\n%s", startLine)
	}
	// Cleanup: container removed + volume dropped.
	if !strings.Contains(calls, "docker rm -f evidra-sbx-run42") || !strings.Contains(calls, "docker volume rm evidra-agent-run42") {
		t.Fatalf("cleanup missing in:\n%s", calls)
	}
}

func TestSandboxStagingUsesNamedVolumeTar(t *testing.T) {
	dir := t.TempDir()
	logPath, _ := fakeDocker(t, dir)
	s := &DockerSandbox{}
	if _, err := s.Run(context.Background(), testSpec(t), []string{"true"}); err != nil {
		t.Fatal(err)
	}
	calls := readCalls(t, logPath)
	if !strings.Contains(calls, "docker volume create evidra-agent-run42") {
		t.Fatalf("no volume create:\n%s", calls)
	}
	var staged bool
	for _, l := range strings.Split(calls, "\n") {
		if strings.Contains(l, "docker run --rm -i -v evidra-agent-run42:/mnt/evidra") && strings.Contains(l, "tar x -C") {
			staged = true
		}
	}
	if !staged {
		t.Fatalf("no tar staging call:\n%s", calls)
	}
}

func TestSandboxCleanupOnStartFailure(t *testing.T) {
	dir := t.TempDir()
	logPath, _ := fakeDocker(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "fail-start"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	s := &DockerSandbox{}
	if _, err := s.Run(context.Background(), testSpec(t), []string{"true"}); err == nil {
		t.Fatal("expected start failure")
	}
	calls := readCalls(t, logPath)
	if !strings.Contains(calls, "docker volume rm evidra-agent-run42") {
		t.Fatalf("volume not cleaned after failed start:\n%s", calls)
	}
}

func TestSandboxExitCodePassthrough(t *testing.T) {
	dir := t.TempDir()
	fakeDocker(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "exec-code"), []byte("3"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &DockerSandbox{}
	res, err := s.Run(context.Background(), testSpec(t), []string{"agent"})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 3 {
		t.Fatalf("ExitCode = %d, want 3 (res %+v)", res.ExitCode, res)
	}
	if !strings.Contains(res.Stdout, "agent stdout line") {
		t.Fatalf("stdout = %q", res.Stdout)
	}
}

func TestSandboxBuildTarLayout(t *testing.T) {
	s := &DockerSandbox{}
	data, err := s.buildTar(testSpec(t))
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(bytes.NewReader(data))
	entries := map[string]int64{}
	for {
		h, err := tr.Next()
		if err != nil {
			break
		}
		entries[h.Name] = h.Mode
	}
	if entries["agent/run"]&0o111 == 0 {
		t.Fatalf("entrypoint must keep exec bit: %v", entries)
	}
	if _, ok := entries["agent/prompt.md"]; !ok {
		t.Fatalf("prompt not staged: %v", entries)
	}
	if _, ok := entries["run/agent.kubeconfig"]; !ok {
		t.Fatalf("kubeconfig not staged: %v", entries)
	}
	if _, ok := entries["inputs/notes.txt"]; !ok {
		t.Fatalf("whitelisted input not staged: %v", entries)
	}
}

func TestDockerNetworkOfEmptyOnFailure(t *testing.T) {
	// No such container -> "" (sandbox unavailable is honest, guessed
	// network names are what bit us).
	if got := DockerNetworkOf("evidra-does-not-exist-xyz"); got != "" {
		t.Fatalf("inspect of missing container must yield empty, got %q", got)
	}
}

func TestBundlePackingRejectsSymlinksAndSpecials(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "bundle")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "run"), []byte("#!/bin/sh\ntrue\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := &DockerSandbox{}
	if _, err := s.buildTar(SandboxSpec{BundleDir: bundle}); err != nil {
		t.Fatalf("plain bundle must pack: %v", err)
	}

	// symlink to a file OUTSIDE the bundle (classic escape): must error
	secret := filepath.Join(root, "host-secret")
	if err := os.WriteFile(secret, []byte("TOP-SK"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(bundle, "innocent.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.buildTar(SandboxSpec{BundleDir: bundle}); err == nil {
		t.Fatal("symlinked bundle entry must refuse packing")
	} else if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("error must name the symlink, got: %v", err)
	}
	if err := os.Remove(filepath.Join(bundle, "innocent.txt")); err != nil {
		t.Fatal(err)
	}

	// symlinked directory must not be walked either
	if err := os.MkdirAll(filepath.Join(root, "outsidedir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "outsidedir"), filepath.Join(bundle, "dirlink")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.buildTar(SandboxSpec{BundleDir: bundle}); err == nil {
		t.Fatal("symlinked directory must refuse packing")
	}
}
