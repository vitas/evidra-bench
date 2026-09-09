package environment

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDetectContainerNetworkMode(t *testing.T) {
	tests := []struct {
		name     string
		inspect  string
		runnerID string
		want     ContainerNetworkMode
	}{
		{name: "host", inspect: "host\n", runnerID: "runner-container", want: ContainerNetworkHost},
		{name: "bridge", inspect: "bridge\n", runnerID: "runner-container", want: ContainerNetworkBridge},
		{name: "default alias", inspect: "default", runnerID: "runner-container", want: ContainerNetworkBridge},
		{name: "custom network stays bridge semantics", inspect: "shared-net\n", runnerID: "runner-container", want: ContainerNetworkBridge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &stubRunner{outputs: map[string][]byte{
				"docker inspect --format {{.HostConfig.NetworkMode}} " + tt.runnerID: []byte(tt.inspect),
			}}
			mode, err := DetectContainerNetworkMode(context.Background(), runner, tt.runnerID)
			if err != nil {
				t.Fatalf("DetectContainerNetworkMode() error = %v", err)
			}
			if mode != tt.want {
				t.Fatalf("mode = %q, want %q", mode, tt.want)
			}
			if len(runner.seen) != 1 {
				t.Fatalf("commands = %v", runner.seen)
			}
		})
	}
}

func TestDetectContainerNetworkModeWithoutContainerIsNative(t *testing.T) {
	runner := &stubRunner{}
	mode, err := DetectContainerNetworkMode(context.Background(), runner, "  ")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if mode != ContainerNetworkNative {
		t.Fatalf("mode = %q, want native", mode)
	}
	if len(runner.seen) != 0 {
		t.Fatalf("no container means no docker calls, got %v", runner.seen)
	}
}

func TestDetectContainerNetworkModeFailureIsActionable(t *testing.T) {
	runner := &stubRunner{outputs: map[string][]byte{
		"docker inspect --format {{.HostConfig.NetworkMode}} runner-container": []byte("Cannot connect to the Docker daemon"),
	}, errs: map[string]error{
		"docker inspect --format {{.HostConfig.NetworkMode}} runner-container": errors.New("exit status 1"),
	}}
	_, err := DetectContainerNetworkMode(context.Background(), runner, "runner-container")
	if err == nil {
		t.Fatal("expected error")
	}
	text := err.Error()
	for _, want := range []string{"runner container", "docker"} {
		if !strings.Contains(strings.ToLower(text), want) {
			t.Fatalf("error %q must mention %q", text, want)
		}
	}
}

func TestDetectContainerNetworkModeRejectsUnparsableOutput(t *testing.T) {
	runner := &stubRunner{outputs: map[string][]byte{
		"docker inspect --format {{.HostConfig.NetworkMode}} runner-container": []byte("\n"),
	}}
	if _, err := DetectContainerNetworkMode(context.Background(), runner, "runner-container"); err == nil {
		t.Fatal("expected malformed-mode error")
	}
}

func TestFailingClusterLifecycleSurfacesEveryCall(t *testing.T) {
	sentinel := errors.New("detection failed")
	lifecycle := &FailingClusterLifecycle{Err: sentinel}
	ctx := context.Background()
	if _, err := lifecycle.Create(ctx, "x", ClusterSpec{}); !errors.Is(err, sentinel) {
		t.Fatalf("Create error = %v", err)
	}
	if err := lifecycle.Destroy(ctx, nil); !errors.Is(err, sentinel) {
		t.Fatalf("Destroy error = %v", err)
	}
	if _, err := lifecycle.Recreate(ctx, "x", ClusterSpec{}); !errors.Is(err, sentinel) {
		t.Fatalf("Recreate error = %v", err)
	}
}
