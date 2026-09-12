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

// Default sandboxing for external agents (release review findings #2 and
// round-4 #1).
//
// An `--agent` command used to run directly in the runner container —
// which holds the docker socket — and still print a clean PASS. Now the
// external agent is wrapped into a synthetic bundle and executed inside
// the same hardened sibling container the explicit-bundle path uses. The
// contract this file guarantees (matching what the docs promise for the
// unconfined adapter):
//
//   - The FIRST token of the command is resolved as a local executable
//     (explicit path or PATH lookup) and COPIED into the bundle, so
//     `--agent "./my-agent --config agent.yaml"` really runs my-agent
//     under the arguments, not a half-broken shell wrapper.
//   - Extra files the agent needs (--agent-input path...) are staged into
//     the bundle next to the entrypoint; the wrapper runs with the bundle
//     directory as cwd, so `--config agent.yaml` resolves.
//   - A command whose first token is NOT a local executable is refused
//     unless an explicit custom --agent-image provides it: we never guess
//     that a binary "might exist" somewhere.
//   - Writable /workspace plus INFRA_BENCH_SCENARIO / INFRA_BENCH_PROMPT /
//     INFRA_BENCH_WORKSPACE are provided (assembled in runAgentSandboxed);
//     credentials cross only through the explicit --agent-env NAME
//     allowlist.
//   - A required-but-unavailable sandbox is INCOMPLETE (sandbox_unavailable),
//     never a silent fallback to unconfined execution.

// runExternalAgentSandboxed wraps cfg.AgentCommand as a synthetic bundle
// and runs it through the standard sandbox.
func (h *Harness) runExternalAgentSandboxed(ctx context.Context, req RunRequest, s *scenario.Scenario, kubeconfigPath, promptContent string, timeout time.Duration) (*adapter.RunResult, error) {
	image, err := h.resolveAgentSandboxImage(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("%w: default agent sandbox: %v", environment.ErrSandboxUnavailable, err)
	}
	bundleDir, err := syntheticAgentBundle(req.Config.AgentCommand, req.Config.AgentInputFiles, req.Config.AgentImage != "")
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

// syntheticAgentBundle turns an external agent command plus declared
// input files into a bundle directory with an executable ./run.
func syntheticAgentBundle(command string, inputs []string, customImage bool) (string, error) {
	args, err := splitAgentCommand(command)
	if err != nil {
		return "", err
	}
	if len(args) == 0 {
		return "", fmt.Errorf("empty agent command")
	}
	dir, err := os.MkdirTemp("", "evidra-agent-bundle-")
	if err != nil {
		return "", err
	}
	fail := func(cause error) (string, error) {
		_ = os.RemoveAll(dir)
		return "", cause
	}

	entry := filepath.Join(dir, "run")
	var wrapper string
	if exe, resolveErr := resolveAgentExecutable(args[0]); resolveErr == nil {
		data, readErr := os.ReadFile(exe)
		if readErr != nil {
			return fail(readErr)
		}
		name := filepath.Base(exe)
		binDir := filepath.Join(dir, "bin")
		if mkErr := os.MkdirAll(binDir, 0o755); mkErr != nil {
			return fail(mkErr)
		}
		if writeErr := os.WriteFile(filepath.Join(binDir, name), data, 0o755); writeErr != nil {
			return fail(writeErr)
		}
		quoted := make([]string, 0, len(args)-1)
		for _, a := range args[1:] {
			quoted = append(quoted, shellQuote(a))
		}
		wrapper = fmt.Sprintf(
			"#!/bin/sh\ncd \"$(dirname \"$0\")\" || exit 1\nexec /mnt/evidra/agent/bin/%s %s\"$@\"\n",
			shellQuote(name), strings.Join(append(quoted, " "), " "))
	} else if customImage {
		// An explicit --agent-image is a stated contract: the image
		// provides the binary, so exec the command verbatim inside it.
		// The cd is still REQUIRED: --agent-input files stage into the
		// bundle dir, not the image's WORKDIR (round-5 finding #1).
		wrapper = "#!/bin/sh\ncd \"$(dirname \"$0\")\" || exit 1\nexec " + command + " \"$@\"\n"
	} else {
		return fail(fmt.Errorf(
			"agent executable %q not found locally (checked PATH). Declare where it lives: stage the binary with --agent-input, provide an image that contains it via --agent-image, use a real --agent-bundle, or run --agent-unconfined explicitly: %v",
			args[0], resolveErr))
	}
	for _, in := range inputs {
		abs, aErr := filepath.Abs(in)
		if aErr != nil {
			return fail(aErr)
		}
		data, rErr := os.ReadFile(abs)
		if rErr != nil {
			return fail(fmt.Errorf("agent input %q: %w", in, rErr))
		}
		dest := filepath.Join(dir, filepath.Base(abs))
		if _, existsErr := os.Stat(dest); existsErr == nil {
			return fail(fmt.Errorf("agent input %q collides with bundle entry %q", in, filepath.Base(abs)))
		}
		if wErr := os.WriteFile(dest, data, 0o644); wErr != nil {
			return fail(wErr)
		}
	}
	if wErr := os.WriteFile(entry, []byte(wrapper), 0o755); wErr != nil {
		return fail(wErr)
	}
	return dir, nil
}

// resolveAgentExecutable finds the command's first token as a regular
// executable file: path-like tokens are taken as given, bare names go
// through PATH.
func resolveAgentExecutable(token string) (string, error) {
	if strings.ContainsAny(token, "/\\") {
		abs, err := filepath.Abs(token)
		if err != nil {
			return "", err
		}
		st, err := os.Stat(abs)
		if err != nil {
			return "", err
		}
		if !st.Mode().IsRegular() || st.Mode()&0o111 == 0 {
			return "", fmt.Errorf("%s is not a regular executable file", abs)
		}
		return abs, nil
	}
	return exec.LookPath(token)
}

// splitAgentCommand performs minimal POSIX-style field splitting
// (single quotes, double quotes, backslash escapes) — enough to take
// `./my-agent --config "my agent.yaml"` apart correctly.
func splitAgentCommand(command string) ([]string, error) {
	var out []string
	var cur strings.Builder
scan:
	for i := 0; i < len(command); {
		switch c := command[i]; c {
		case ' ', '\t', '\n':
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
			i++
		case '\\':
			if i+1 >= len(command) {
				return nil, fmt.Errorf("trailing escape in agent command")
			}
			i++
			cur.WriteByte(command[i])
			i++
		case '\'':
			i++
			for {
				if i >= len(command) {
					return nil, fmt.Errorf("unterminated single quote in agent command")
				}
				if command[i] == '\'' {
					i++
					continue scan
				}
				cur.WriteByte(command[i])
				i++
			}
		case '"':
			i++
			for {
				if i >= len(command) {
					return nil, fmt.Errorf("unterminated double quote in agent command")
				}
				if command[i] == '"' {
					i++
					continue scan
				}
				if command[i] == '\\' && i+1 < len(command) {
					i++
				}
				cur.WriteByte(command[i])
				i++
			}
		default:
			cur.WriteByte(c)
			i++
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out, nil
}

// shellQuote wraps a field for re-embedding inside /bin/sh scripts.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
