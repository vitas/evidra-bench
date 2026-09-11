package harness

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/vitas/evidra-bench/pkg/adapter"
	"github.com/vitas/evidra-bench/pkg/environment"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

// Default sandboxing for external agents (release review finding #2).
//
// An `--agent` command used to run directly in the runner container —
// which holds the docker socket — and still print a clean PASS. Now the
// external agent is wrapped into a synthetic bundle and executed inside
// the same hardened sibling container the bundle path uses. The runner's
// own image doubles as the sandbox image (it ships bash + kubectl) unless
// EVIDRA_AGENT_SANDBOX_IMAGE or --agent-image overrides it. Opting out
// (--agent-unconfined) is honest and loud: the case carries the
// agent_unconfined_execution gap and profiled scenarios grade INCOMPLETE.

// runExternalAgentSandboxed wraps cfg.AgentCommand as a synthetic bundle
// and runs it through the standard sandbox. A required-but-unavailable
// sandbox is an infrastructure error (INCOMPLETE), never a silent
// fallback to unconfined execution.
func (h *Harness) runExternalAgentSandboxed(ctx context.Context, req RunRequest, s *scenario.Scenario, kubeconfigPath, promptContent string, timeout time.Duration) (*adapter.RunResult, error) {
	image, err := h.resolveAgentSandboxImage(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("%w: default agent sandbox: %v", environment.ErrSandboxUnavailable, err)
	}
	bundleDir, err := syntheticAgentBundle(req.Config.AgentCommand)
	if err != nil {
		return nil, fmt.Errorf("harness: synthetic agent bundle: %w", err)
	}
	defer func() { _ = os.RemoveAll(bundleDir) }()

	sbReq := req
	sbReq.Config.AgentImage = image
	sbReq.Config.AgentBundleDir = bundleDir
	res, err := h.runAgentSandboxed(ctx, sbReq, s, kubeconfigPath, promptContent, timeout)
	if res != nil {
		if res.Metadata == nil {
			res.Metadata = map[string]string{}
		}
		res.Metadata["agent_mode"] = "external-sandboxed"
	}
	return res, err
}

// resolveAgentSandboxImage picks the image for the default agent sandbox:
// an explicit --agent-image wins, then EVIDRA_AGENT_SANDBOX_IMAGE, then
// the runner's own image discovered through the mounted docker socket.
func (h *Harness) resolveAgentSandboxImage(ctx context.Context, req RunRequest) (string, error) {
	if img := strings.TrimSpace(req.Config.AgentImage); img != "" {
		return img, nil
	}
	if img := strings.TrimSpace(os.Getenv("EVIDRA_AGENT_SANDBOX_IMAGE")); img != "" {
		return img, nil
	}
	containerID := strings.TrimSpace(os.Getenv("HOSTNAME"))
	if containerID == "" {
		return "", fmt.Errorf("not running in a container (no HOSTNAME); pass --agent-image or set EVIDRA_AGENT_SANDBOX_IMAGE to sandbox the external agent")
	}
	out, err := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.Config.Image}}", containerID).Output()
	img := strings.TrimSpace(string(out))
	if err != nil || img == "" || img == "<no value>" {
		return "", fmt.Errorf("runner image not discoverable via docker inspect %s (pass --agent-image or set EVIDRA_AGENT_SANDBOX_IMAGE): %v", containerID, err)
	}
	return img, nil
}

// syntheticAgentBundle turns an external agent command into a bundle
// directory with an executable ./run. A single executable path is copied
// into the bundle (the sandbox sees staged content, not this filesystem);
// anything else is wrapped in a shell exec line.
func syntheticAgentBundle(command string) (string, error) {
	dir, err := os.MkdirTemp("", "evidra-agent-bundle-")
	if err != nil {
		return "", err
	}
	entry := filepath.Join(dir, "run")
	fields := strings.Fields(command)
	if len(fields) == 1 {
		if st, statErr := os.Stat(fields[0]); statErr == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			data, readErr := os.ReadFile(fields[0])
			if readErr != nil {
				return "", readErr
			}
			if writeErr := os.WriteFile(entry, data, 0o755); writeErr != nil {
				return "", writeErr
			}
			return dir, nil
		}
	}
	wrapped := "#!/bin/sh\nexec " + command + " \"$@\"\n"
	if err := os.WriteFile(entry, []byte(wrapped), 0o755); err != nil {
		return "", err
	}
	return dir, nil
}
