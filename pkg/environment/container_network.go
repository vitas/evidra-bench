package environment

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// ContainerNetworkMode describes how the runner container reaches cluster
// API servers. It is detected once from the live container, never assumed
// from environment flags alone.
type ContainerNetworkMode string

const (
	// ContainerNetworkNative: the process runs outside a runner container.
	ContainerNetworkNative ContainerNetworkMode = "native"
	// ContainerNetworkBridge: classic Docker bridge; the runner is attached
	// to the cluster's Docker network and uses internally resolvable
	// kubeconfig endpoints.
	ContainerNetworkBridge ContainerNetworkMode = "bridge"
	// ContainerNetworkHost: the runner shares the host network namespace
	// (docker run --network host). Host-published API ports are reachable
	// directly; bridge attachment would be wrong or harmful.
	ContainerNetworkHost ContainerNetworkMode = "host"
)

// DetectContainerNetworkMode inspects the runner container through the Docker
// API socket using the CommandRunner seam. An empty container name means the
// process is not containerized. Detection is read-only.
func DetectContainerNetworkMode(ctx context.Context, runner CommandRunner, containerName string) (ContainerNetworkMode, error) {
	if strings.TrimSpace(containerName) == "" {
		return ContainerNetworkNative, nil
	}
	if runner == nil {
		runner = &ExecRunner{}
	}
	out, err := runner.Run(ctx, exec.Command("docker", "inspect", "--format", "{{.HostConfig.NetworkMode}}", containerName))
	if err != nil {
		return "", fmt.Errorf("environment: cannot inspect runner container %q (is the Docker socket available inside the runner?): %w: %s",
			containerName, err, strings.TrimSpace(string(out)))
	}
	switch strings.TrimSpace(string(out)) {
	case "host":
		return ContainerNetworkHost, nil
	case "bridge", "default":
		return ContainerNetworkBridge, nil
	case "":
		return "", fmt.Errorf("environment: docker inspect returned no network mode for runner container %q", containerName)
	default:
		// Any named user-defined network behaves like a bridge for our
		// purposes: attach the runner to the cluster network.
		return ContainerNetworkBridge, nil
	}
}

// FailingClusterLifecycle stands in for real providers when runner container
// inspection failed. It fails every lifecycle call with the detection error,
// which happens strictly before any cluster is created.
type FailingClusterLifecycle struct {
	Err error
}

func (f *FailingClusterLifecycle) Create(context.Context, string, ClusterSpec) (*Handle, error) {
	return nil, f.err()
}

func (f *FailingClusterLifecycle) Destroy(context.Context, *Handle) error { return f.err() }

func (f *FailingClusterLifecycle) Recreate(context.Context, string, ClusterSpec) (*Handle, error) {
	return nil, f.err()
}

func (f *FailingClusterLifecycle) HealthCheck(context.Context, string) error { return f.err() }

func (f *FailingClusterLifecycle) ForceDeleteNamespace(context.Context, string, string) error {
	return f.err()
}

func (f *FailingClusterLifecycle) CreateNamespace(context.Context, string, string) error {
	return f.err()
}

func (f *FailingClusterLifecycle) RunCanary(context.Context, string, string) error { return f.err() }

func (f *FailingClusterLifecycle) err() error {
	if f.Err == nil {
		return fmt.Errorf("environment: cluster lifecycle unavailable")
	}
	return f.Err
}
