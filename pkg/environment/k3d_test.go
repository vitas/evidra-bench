package environment

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestK3dProvider_HostNetworkKeepsPublishedLocalPort(t *testing.T) {
	runner := &stubRunner{outputs: map[string][]byte{
		"k3d cluster list --no-headers":               {},
		"k3d cluster create host-test --no-lb --wait": {},
		"k3d kubeconfig get host-test": []byte(`apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: preserved
    server: https://host.docker.internal:49123
  name: k3d-host-test
kind: Config
`),
	}}
	p := &K3dProvider{
		kubectlOps:    kubectlOps{Runner: runner},
		ContainerName: "runner-container",
		NetworkMode:   ContainerNetworkHost,
	}

	handle, err := p.Create(context.Background(), "host-test", ClusterSpec{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer func() { _ = os.Remove(handle.KubeconfigPath) }()

	for _, command := range runner.seen {
		if strings.Contains(command, "network connect") {
			t.Fatalf("host networking must not attach to bridge networks: %v", runner.seen)
		}
	}
	written, err := os.ReadFile(handle.KubeconfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "server: https://127.0.0.1:49123") {
		t.Fatalf("host mode must localize the published API port, got: %s", written)
	}
	if !strings.Contains(string(written), "certificate-authority-data: preserved") {
		t.Fatalf("credentials must be preserved: %s", written)
	}
}

func TestK3dProvider_HostNetworkRecreateNeverDisconnectsBridgeNetwork(t *testing.T) {
	runner := &stubRunner{outputs: map[string][]byte{
		"k3d cluster delete host-test":                {},
		"k3d cluster list --no-headers":               {},
		"k3d cluster create host-test --no-lb --wait": {},
		"k3d kubeconfig get host-test": []byte(`apiVersion: v1
clusters:
- cluster:
    server: https://0.0.0.0:49123
  name: k3d-host-test
kind: Config
`),
	}}
	p := &K3dProvider{
		kubectlOps:    kubectlOps{Runner: runner},
		ContainerName: "runner-container",
		NetworkMode:   ContainerNetworkHost,
	}

	handle, err := p.Recreate(context.Background(), "host-test", ClusterSpec{})
	if err != nil {
		t.Fatalf("Recreate() error = %v", err)
	}
	defer func() { _ = os.Remove(handle.KubeconfigPath) }()
	for _, command := range runner.seen {
		if strings.Contains(command, "network disconnect") || strings.Contains(command, "network connect") {
			t.Fatalf("host-network recreate touched a bridge network: %v", runner.seen)
		}
	}
}

func TestK3dProvider_CreateCommand(t *testing.T) {
	t.Parallel()
	p := NewK3dProvider()
	cmd := p.createCommand("bench-test")
	got := strings.Join(cmd.Args, " ")
	if !strings.Contains(got, "k3d cluster create") {
		t.Fatalf("unexpected command: %s", got)
	}
	if !strings.Contains(got, "bench-test") {
		t.Fatalf("missing cluster name: %s", got)
	}
}

func TestK3dProvider_DeleteCommand(t *testing.T) {
	t.Parallel()
	p := NewK3dProvider()
	cmd := p.deleteCommand("bench-test")
	got := strings.Join(cmd.Args, " ")
	if !strings.Contains(got, "k3d cluster delete") {
		t.Fatalf("unexpected command: %s", got)
	}
	if !strings.Contains(got, "bench-test") {
		t.Fatalf("missing cluster name: %s", got)
	}
}

func TestK3dProvider_KubeconfigCommand(t *testing.T) {
	t.Parallel()
	p := NewK3dProvider()
	cmd := p.kubeconfigCommand("bench-test")
	got := strings.Join(cmd.Args, " ")
	if !strings.Contains(got, "k3d kubeconfig get") {
		t.Fatalf("unexpected command: %s", got)
	}
}

func TestK3dProvider_ContainerUsesClusterNetworkAndInternalKubeconfig(t *testing.T) {
	runner := &stubRunner{outputs: map[string][]byte{
		"k3d cluster list --no-headers":                           {},
		"k3d cluster create docker-test --no-lb --wait":           {},
		"docker network connect k3d-docker-test runner-container": {},
		"k3d kubeconfig get docker-test": []byte(`apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: preserved
    server: https://0.0.0.0:49123
  name: k3d-docker-test
kind: Config
`),
	}}
	p := &K3dProvider{
		kubectlOps:    kubectlOps{Runner: runner},
		ContainerName: "runner-container",
	}

	handle, err := p.Create(context.Background(), "docker-test", ClusterSpec{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer func() { _ = os.Remove(handle.KubeconfigPath) }()

	written, err := os.ReadFile(handle.KubeconfigPath)
	if err != nil {
		t.Fatalf("read kubeconfig: %v", err)
	}
	if !strings.Contains(string(written), "server: https://k3d-docker-test-server-0:6443") {
		t.Fatalf("kubeconfig = %s, want internal k3d server", written)
	}
	if !strings.Contains(string(written), "certificate-authority-data: preserved") {
		t.Fatalf("kubeconfig = %s, want existing credentials preserved", written)
	}

	wantOrder := []string{
		"k3d cluster create docker-test --no-lb --wait",
		"docker network connect k3d-docker-test runner-container",
		"k3d kubeconfig get docker-test",
	}
	position := 0
	for _, command := range runner.seen {
		if position < len(wantOrder) && command == wantOrder[position] {
			position++
		}
	}
	if position != len(wantOrder) {
		t.Fatalf("commands = %v, want ordered subsequence %v", runner.seen, wantOrder)
	}
}

func TestK3dProvider_DestroyDisconnectsRunnerBeforeDeletingCluster(t *testing.T) {
	runner := &stubRunner{outputs: map[string][]byte{}}
	p := &K3dProvider{
		kubectlOps:    kubectlOps{Runner: runner},
		ContainerName: "runner-container",
	}
	handle := &Handle{ClusterName: "docker-test", KubeconfigPath: t.TempDir() + "/kubeconfig"}

	if err := p.Destroy(context.Background(), handle); err != nil {
		t.Fatalf("Destroy() error = %v", err)
	}
	want := []string{
		"docker network disconnect k3d-docker-test runner-container",
		"k3d cluster delete docker-test",
	}
	if strings.Join(runner.seen, "|") != strings.Join(want, "|") {
		t.Fatalf("commands = %v, want %v", runner.seen, want)
	}
}

func TestK3dProvider_ImplementsProvider(t *testing.T) {
	t.Parallel()
	var _ ClusterLifecycle = (*K3dProvider)(nil)
}

func TestK3dProvider_RunCanaryUsesK3sPauseImageWhenNodeImagesAreEmpty(t *testing.T) {
	t.Parallel()
	runner := &seqRunner{responses: []seqResponse{
		{out: []byte("")},
		{out: []byte("")},
		{out: []byte("pod/bench-canary created")},
		{out: []byte("pod/bench-canary condition met")},
		{out: []byte("pod/bench-canary deleted")},
	}}
	p := &K3dProvider{kubectlOps: kubectlOps{Runner: runner}}

	if err := p.RunCanary(context.Background(), "/tmp/kc", "bench"); err != nil {
		t.Fatalf("RunCanary() error = %v", err)
	}
	if !strings.Contains(runner.calls[2], "docker.io/rancher/mirrored-pause:3.6") {
		t.Fatalf("canary create command = %q, want preloaded k3s pause image", runner.calls[2])
	}
}

func TestK3dProvider_Create_ReusesExistingCluster(t *testing.T) {
	t.Parallel()

	runner := &stubRunner{
		outputs: map[string][]byte{
			"k3d cluster list --no-headers": []byte("bench-cli   1/1   0/0   true\n"),
			"k3d kubeconfig get bench-cli":  []byte("apiVersion: v1\nkind: Config\n"),
		},
	}
	p := &K3dProvider{
		kubectlOps:    kubectlOps{Runner: runner},
		ReuseExisting: true,
	}

	handle, err := p.Create(context.Background(), "bench-cli", ClusterSpec{})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if handle.ClusterName != "bench-cli" {
		t.Fatalf("unexpected cluster: %s", handle.ClusterName)
	}
	for _, cmd := range runner.seen {
		if strings.Contains(cmd, "k3d cluster create") {
			t.Fatalf("unexpected create command when reusing cluster: %s", cmd)
		}
	}
}

type k3dRunner struct {
	results map[string]struct {
		out []byte
		err error
	}
	seen []string
}

func (r *k3dRunner) Run(_ context.Context, cmd *exec.Cmd) ([]byte, error) {
	key := strings.Join(cmd.Args, " ")
	r.seen = append(r.seen, key)
	if result, ok := r.results[key]; ok {
		return result.out, result.err
	}
	return nil, nil
}

func TestK3dProvider_Create_UsesExplicitConfigPath(t *testing.T) {
	t.Parallel()

	runner := &stubRunner{
		outputs: map[string][]byte{
			"k3d cluster list --no-headers":                                        []byte(""),
			"k3d cluster create bench-cli --config /repo/clusters/k3d/argocd.yaml": []byte(""),
			"k3d kubeconfig get bench-cli":                                         []byte("apiVersion: v1\nkind: Config\n"),
		},
	}
	p := &K3dProvider{
		kubectlOps: kubectlOps{Runner: runner},
	}

	spec := ClusterSpec{ConfigPath: "/repo/clusters/k3d/argocd.yaml"}
	handle, err := p.Create(context.Background(), "bench-cli", spec)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if handle.ClusterName != "bench-cli" {
		t.Fatalf("unexpected cluster: %s", handle.ClusterName)
	}

	found := false
	for _, cmd := range runner.seen {
		if strings.Contains(cmd, "--config /repo/clusters/k3d/argocd.yaml") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected --config flag with explicit path, got commands: %v", runner.seen)
	}
}

func TestK3dProvider_Create_FallsBackToPlainClusterWhenSpecEmpty(t *testing.T) {
	t.Parallel()

	runner := &stubRunner{
		outputs: map[string][]byte{
			"k3d cluster list --no-headers":               []byte(""),
			"k3d cluster create bench-cli --no-lb --wait": []byte(""),
			"k3d kubeconfig get bench-cli":                []byte("apiVersion: v1\nkind: Config\n"),
		},
	}
	p := &K3dProvider{
		kubectlOps: kubectlOps{Runner: runner},
	}

	handle, err := p.Create(context.Background(), "bench-cli", ClusterSpec{})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if handle.ClusterName != "bench-cli" {
		t.Fatalf("unexpected cluster: %s", handle.ClusterName)
	}

	for _, cmd := range runner.seen {
		if strings.Contains(cmd, "--config") {
			t.Fatalf("unexpected --config flag for empty spec: %s", cmd)
		}
	}
}

func TestK3dProvider_CreateIncludesCommandOutputOnFailure(t *testing.T) {
	t.Parallel()

	runner := &k3dRunner{results: map[string]struct {
		out []byte
		err error
	}{
		"k3d cluster create bench-cli --no-lb --wait": {
			out: []byte("specific k3d failure"),
			err: errors.New("exit status 1"),
		},
	}}
	p := &K3dProvider{kubectlOps: kubectlOps{Runner: runner}}

	_, err := p.Create(context.Background(), "bench-cli", ClusterSpec{})
	if err == nil {
		t.Fatal("Create() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "specific k3d failure") {
		t.Fatalf("Create() error = %v, want command output", err)
	}
}

func TestK3dProvider_Recreate_DeleteFailureIsFatal(t *testing.T) {
	t.Parallel()

	runner := &k3dRunner{
		results: map[string]struct {
			out []byte
			err error
		}{
			"k3d cluster delete bench-cli": {
				out: []byte("delete failed"),
				err: errors.New("exit status 1"),
			},
		},
	}
	p := &K3dProvider{
		kubectlOps:    kubectlOps{Runner: runner},
		ReuseExisting: true,
	}

	_, err := p.Recreate(context.Background(), "bench-cli", ClusterSpec{})
	if err == nil {
		t.Fatal("Recreate() error = nil, want delete failure")
	}
	if !strings.Contains(err.Error(), "delete existing cluster") && !strings.Contains(err.Error(), "delete during recreate") {
		t.Fatalf("Recreate() error = %v, want delete context", err)
	}
	for _, cmd := range runner.seen {
		if strings.Contains(cmd, "k3d cluster create") {
			t.Fatalf("unexpected create command after delete failure: %s", cmd)
		}
	}
}
