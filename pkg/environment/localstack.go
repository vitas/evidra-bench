package environment

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

const localstackImage = "localstack/localstack:3.8"
const localstackPort = "4566"

// LocalStackHandle holds references to a running LocalStack instance.
type LocalStackHandle struct {
	ContainerID string
	EndpointURL string
}

// StartLocalStack starts a LocalStack container and waits for readiness.
func StartLocalStack(ctx context.Context, name string, services []string) (*LocalStackHandle, error) {
	containerName := fmt.Sprintf("localstack-%s", name)

	// Pull image (best-effort, may already exist).
	_ = exec.CommandContext(ctx, "docker", "pull", localstackImage).Run()

	// Remove any existing container with this name.
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", containerName).Run()

	// Start LocalStack.
	args := []string{
		"run", "-d",
		"--name", containerName,
		"-p", fmt.Sprintf("%s:%s", localstackPort, localstackPort),
		"-e", fmt.Sprintf("SERVICES=%s", strings.Join(services, ",")),
		"-e", "DEBUG=0",
		"-e", "DOCKER_HOST=unix:///var/run/docker.sock",
		localstackImage,
	}
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("environment.StartLocalStack: %s: %w", string(out), err)
	}

	containerID := strings.TrimSpace(string(out))
	endpointURL := fmt.Sprintf("http://localhost:%s", localstackPort)

	// Wait for readiness.
	if err := waitForLocalStack(ctx, endpointURL); err != nil {
		_ = exec.CommandContext(ctx, "docker", "rm", "-f", containerName).Run()
		return nil, err
	}

	log.Printf("[localstack] started %s at %s (services: %v)", containerName, endpointURL, services)

	return &LocalStackHandle{
		ContainerID: containerID,
		EndpointURL: endpointURL,
	}, nil
}

// StopLocalStack stops and removes a LocalStack container.
func StopLocalStack(ctx context.Context, handle *LocalStackHandle) error {
	if handle == nil || handle.ContainerID == "" {
		return nil
	}
	out, err := exec.CommandContext(ctx, "docker", "rm", "-f", handle.ContainerID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("environment.StopLocalStack: %s: %w", string(out), err)
	}
	log.Printf("[localstack] stopped %s", handle.ContainerID)
	return nil
}

func waitForLocalStack(ctx context.Context, endpointURL string) error {
	deadline := time.After(60 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("environment.StartLocalStack: readiness timeout after 60s")
		case <-ticker.C:
			resp, err := http.Get(endpointURL + "/_localstack/health")
			if err == nil && resp.StatusCode == 200 {
				resp.Body.Close()
				return nil
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
	}
}
