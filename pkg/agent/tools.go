package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// BenchTools returns the tool definitions exposed to the LLM.
func BenchTools() []ToolDef {
	return []ToolDef{
		{
			Name:        "run_command",
			Description: "Execute a shell command against the cluster. Use for kubectl, helm, terraform, and other CLI tools. The KUBECONFIG environment variable is already set.",
			Parameters: map[string]any{
				"type":     "object",
				"required": []string{"command"},
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "Shell command to execute (e.g., 'kubectl get pods -n bench')",
					},
				},
			},
		},
		{
			Name:        "write_file",
			Description: "Write content to a file. Use for creating or updating configuration files (e.g., Terraform .tf files, YAML manifests). Creates parent directories if needed.",
			Parameters: map[string]any{
				"type":     "object",
				"required": []string{"path", "content"},
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "File path to write to",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "Content to write to the file",
					},
				},
			},
		},
	}
}

// ToolExecutor runs tool calls against the real environment.
type ToolExecutor struct {
	KubeconfigPath string
	ExtraEnv       []string // Additional env vars for commands (e.g., AWS_ENDPOINT_URL)
}

// Execute runs a single tool call and returns the result string.
func (e *ToolExecutor) Execute(ctx context.Context, tc ToolCall) string {
	switch tc.Name {
	case "run_command":
		return e.runCommand(ctx, tc.Arguments)
	case "write_file":
		return e.writeFile(ctx, tc.Arguments)
	default:
		return fmt.Sprintf("unknown tool: %s", tc.Name)
	}
}

// allowedCommandPrefixes restricts which commands the LLM can execute.
var allowedCommandPrefixes = []string{
	"kubectl", "helm", "argocd", "kind", "terraform", "aws", "kustomize",
	"cat", "echo", "grep", "head", "tail", "wc", "ls", "find", "openssl",
	"jq", "yq", "sed", "tee", "cp", "mkdir", "rm", "mv",
}

var toolCommandTimeout = 60 * time.Second

// blockedSubcommands blocks interactive or dangerous subcommands.
var blockedSubcommands = []string{
	"kubectl edit ",
	"kubectl exec -it ",
	"kubectl exec -ti ",
	"kubectl exec --stdin --tty ",
	"kubectl attach ",
	"kubectl port-forward ",
	"kubectl proxy",
	"kubectl run --stdin ",
	"kubectl run -it ",
	"kubectl run -ti ",
	"helm shell",
	"terraform console",
}

// validateCommand checks that a command starts with an allowed prefix
// and does not match any blocked interactive subcommand.
// Compound commands (using &&, ||, ;, |) are split and each segment is validated.
func validateCommand(command string) error {
	trimmed := strings.TrimSpace(command)

	// Split compound commands into segments.
	segments := splitCompoundCommand(trimmed)
	for _, seg := range segments {
		if err := validateSegment(seg); err != nil {
			return err
		}
	}
	return nil
}

// compoundAllowed are commands allowed only as part of compound commands (not standalone).
var compoundAllowed = []string{"cd"}

// splitCompoundCommand splits a shell command on &&, ||, and ; operators.
// Does NOT split on | (pipe) because it appears inside sed/grep patterns.
func splitCompoundCommand(cmd string) []string {
	// Replace operators with a unique separator, then split.
	// Order matters: check && and || before single characters.
	normalized := cmd
	for _, op := range []string{"&&", "||", ";"} {
		normalized = strings.ReplaceAll(normalized, op, "\x00")
	}
	parts := strings.Split(normalized, "\x00")
	segments := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			segments = append(segments, p)
		}
	}
	return segments
}

func validateSegment(seg string) error {
	// Check blocklist first.
	for _, blocked := range blockedSubcommands {
		if seg == strings.TrimSpace(blocked) || strings.HasPrefix(seg, blocked) {
			return fmt.Errorf("command %q is blocked (interactive/dangerous)", truncate(seg, 60))
		}
	}

	// Block watch mode — these never exit and cause timeouts.
	if strings.HasPrefix(seg, "kubectl ") && (containsFlag(seg, "-w") || containsFlag(seg, "--watch")) {
		return fmt.Errorf("command %q is blocked (watch mode never exits)", truncate(seg, 60))
	}

	// Check main allowlist.
	for _, prefix := range allowedCommandPrefixes {
		if seg == prefix || strings.HasPrefix(seg, prefix+" ") {
			return nil
		}
	}

	// Check compound-only allowlist (cd, etc.).
	for _, prefix := range compoundAllowed {
		if seg == prefix || strings.HasPrefix(seg, prefix+" ") {
			return nil
		}
	}

	return fmt.Errorf("command %q not in allowlist (allowed: %v)", truncate(seg, 50), allowedCommandPrefixes)
}

// containsFlag checks whether flag appears as a standalone word in cmd.
func containsFlag(cmd, flag string) bool {
	padded := " " + cmd + " "
	return strings.Contains(padded, " "+flag+" ") || strings.HasSuffix(" "+cmd, " "+flag)
}

func (e *ToolExecutor) runCommand(ctx context.Context, argsJSON string) string {
	var args struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("error parsing arguments: %v", err)
	}
	if args.Command == "" {
		return "error: command is required"
	}
	if err := validateCommand(args.Command); err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	runCtx := ctx
	cancel := func() {}
	if toolCommandTimeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, toolCommandTimeout)
	}
	defer cancel()

	cmd := exec.CommandContext(runCtx, "bash", "-c", args.Command)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+e.KubeconfigPath)
	cmd.Env = append(cmd.Env, e.ExtraEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := runProcessGroupCommand(runCtx, cmd)

	result := stdout.String()
	if stderr.Len() > 0 {
		result += "\nSTDERR: " + stderr.String()
	}
	ec := 0
	if err != nil {
		ec = exitCode(err)
		if errors.Is(err, context.DeadlineExceeded) {
			result += fmt.Sprintf("\nerror: command timed out after %s", toolCommandTimeout)
		}
		result += fmt.Sprintf("\nExit code: %d", ec)
	}

	return strings.TrimSpace(result)
}

func runProcessGroupCommand(ctx context.Context, cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			_ = cmd.Process.Kill()
		}
		<-done
		return ctx.Err()
	}
}

func (e *ToolExecutor) writeFile(_ context.Context, argsJSON string) string {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("error parsing arguments: %v", err)
	}
	if args.Path == "" {
		return "error: path is required"
	}

	// Create parent directories if needed.
	dir := filepath.Dir(args.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Sprintf("error creating directory %s: %v", dir, err)
	}

	if err := os.WriteFile(args.Path, []byte(args.Content), 0o644); err != nil {
		return fmt.Sprintf("error writing file: %v", err)
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(args.Content), args.Path)
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if ok := errors.As(err, &exitErr); ok {
		return exitErr.ExitCode()
	}
	return 1
}
