package harness

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/vitas/evidra-bench/pkg/adapter"
	"github.com/vitas/evidra-bench/pkg/environment"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

// runAgentSandboxed executes the agent inside the hardened sibling
// container (ADR 0001 Phase 6). Contract:
//
//   - cfg.AgentImage is mandatory; the image is caller-supplied.
//   - cfg.AgentBundleDir (optional) must contain an executable ./run; it is
//     staged read-only at /mnt/evidra/agent together with the prompt file
//     and the scenario's declared agent_inputs (whitelisted files only —
//     the scenario DIRECTORY is never exposed, it can hold expected data).
//   - The agent identity kubeconfig crosses as staged volume content; the
//     sandbox sees exactly: bundle, inputs, prompt, its own kubeconfig.
//   - A required-but-unavailable sandbox is INCOMPLETE (sandbox_unavailable),
//     never a silent fallback to unconfined execution.
func (h *Harness) runAgentSandboxed(ctx context.Context, req RunRequest, s *scenario.Scenario, kubeconfigPath, promptContent string, timeout time.Duration) (*adapter.RunResult, error) {
	if req.Config.AgentImage == "" {
		return nil, fmt.Errorf("harness: agent bundle %s requires --agent-image (a bundle alone cannot define the execution environment)", req.Config.AgentBundleDir)
	}
	if req.Config.AgentBundleDir == "" {
		return nil, fmt.Errorf("harness: --agent-image requires --agent-bundle with an ./run entrypoint (qualified sandboxed runs must declare their agent explicitly)")
	}
	entry, err := os.Stat(filepath.Join(req.Config.AgentBundleDir, "run"))
	if err != nil || entry.IsDir() || entry.Mode()&0o111 == 0 {
		return nil, fmt.Errorf("harness: agent bundle %s must contain an executable ./run entrypoint", req.Config.AgentBundleDir)
	}
	if req.ClusterNetwork == "" {
		return nil, fmt.Errorf("%w: cluster network unknown (sandbox requires the provisioned cluster's docker network)", environment.ErrSandboxUnavailable)
	}
	sbx := h.deps.Sandbox
	if sbx == nil {
		sbx = &environment.DockerSandbox{}
	}
	if err := sbx.Available(ctx); err != nil {
		return nil, err
	}
	extra, err := readAgentInputs(s)
	if err != nil {
		return nil, err
	}
	spec := environment.SandboxSpec{
		// Timestamp-suffixed and tail-kept: unique per concurrent run
		// even after volume-name shortening.
		RunID:         sanitizeRunID(fmt.Sprintf("%s-%d", s.ID, time.Now().UnixNano())),
		Image:         req.Config.AgentImage,
		BundleDir:     req.Config.AgentBundleDir,
		PromptContent: promptContent,
		Kubeconfig:    kubeconfigPath,
		ExtraFiles:    extra,
		// The scenario id is part of the agent's TASK CONTRACT (the prompt
		// states it); passing it as INFRA_BENCH_SCENARIO mirrors what the
		// unconfined adapter sets and lets multi-case bundles dispatch.
		AgentEnv: map[string]string{"INFRA_BENCH_SCENARIO": s.ID},
		Network:  req.ClusterNetwork,
		Memory:   req.Config.SandboxMemory,
		CPUs:     req.Config.SandboxCPUs,
		Timeout:  timeout,
	}
	argv := []string{"/mnt/evidra/agent/run", "/mnt/evidra/agent/prompt.md"}
	res, err := sbx.Run(ctx, spec, argv)
	if err != nil {
		return nil, err
	}
	out := &adapter.RunResult{
		ExitCode:   res.ExitCode,
		Stdout:     res.Stdout,
		Stderr:     res.Stderr,
		Transcript: res.Stdout + res.Stderr,
		Metadata: map[string]string{
			"sandbox":       "1",
			"sandbox_image": req.Config.AgentImage,
		},
	}
	if errors.Is(err, environment.ErrSandboxUnavailable) {
		return out, err
	}
	return out, nil
}

// readAgentInputs materializes the scenario's declared input whitelist.
func readAgentInputs(s *scenario.Scenario) (map[string]string, error) {
	if len(s.AgentInputs) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(s.AgentInputs))
	for _, rel := range s.AgentInputs {
		path := filepath.Join(s.Dir, filepath.FromSlash(rel))
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("harness: scenario agent_inputs %q: %w", rel, err)
		}
		out[filepath.ToSlash(rel)] = string(data)
	}
	return out, nil
}

func sanitizeRunID(id string) string {
	if id == "" {
		return "run"
	}
	out := make([]rune, 0, len(id))
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			out = append(out, r)
		}
	}
	const maxLen = 40
	if len(out) > maxLen {
		out = out[len(out)-maxLen:] // keep the unique tail
	}
	if len(out) == 0 {
		return "run"
	}
	return string(out)
}
