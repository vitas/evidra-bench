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
	WorkspaceDir   string
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

// validateCommand checks that every shell command segment starts with an allowed
// prefix and does not match any blocked interactive subcommand.
func validateCommand(command string) error {
	trimmed := strings.TrimSpace(command)
	if err := rejectShellSubstitution(trimmed); err != nil {
		return err
	}

	segments, err := splitCompoundCommand(trimmed)
	if err != nil {
		return err
	}
	for _, seg := range segments {
		if err := validateSegment(seg); err != nil {
			return err
		}
	}
	return nil
}

// compoundAllowed are commands allowed only as part of compound commands (not standalone).
var compoundAllowed = []string{"cd"}

// splitCompoundCommand splits shell control operators that can start another
// command. Operators inside quotes are preserved for commands like jq, grep, and sed.
// Background execution is blocked because it can outlive the tool timeout.
func splitCompoundCommand(cmd string) ([]string, error) {
	segments := make([]string, 0, 1)
	start := 0
	inSingle := false
	inDouble := false
	escaped := false

	appendSegment := func(end int) {
		seg := strings.TrimSpace(cmd[start:end])
		if seg != "" {
			segments = append(segments, seg)
		}
	}

	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && !inSingle {
			escaped = true
			continue
		}
		if c == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if c == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if inSingle || inDouble {
			continue
		}

		switch c {
		case '\n', '\r':
			appendSegment(i)
			start = i + 1
		case ';':
			appendSegment(i)
			start = i + 1
		case '&':
			if i+1 < len(cmd) && cmd[i+1] == '&' {
				appendSegment(i)
				i++
				start = i + 1
			} else {
				return nil, fmt.Errorf("command %q uses blocked background execution", truncate(cmd, 60))
			}
		case '|':
			appendSegment(i)
			if i+1 < len(cmd) && cmd[i+1] == '|' {
				i++
			}
			start = i + 1
		}
	}
	if inSingle || inDouble {
		return nil, fmt.Errorf("command %q has an unterminated quote", truncate(cmd, 60))
	}
	appendSegment(len(cmd))
	return segments, nil
}

func rejectShellSubstitution(cmd string) error {
	inSingle := false
	inDouble := false
	escaped := false

	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && !inSingle {
			escaped = true
			continue
		}
		if c == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if c == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if inSingle {
			continue
		}

		if c == '`' {
			return fmt.Errorf("command %q uses blocked shell substitution", truncate(cmd, 60))
		}
		if c == '$' && i+1 < len(cmd) && cmd[i+1] == '(' {
			return fmt.Errorf("command %q uses blocked shell substitution", truncate(cmd, 60))
		}
		if (c == '<' || c == '>') && i+1 < len(cmd) && cmd[i+1] == '(' {
			return fmt.Errorf("command %q uses blocked process substitution", truncate(cmd, 60))
		}
	}
	return nil
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
	if strings.HasPrefix(seg, "find ") &&
		(containsFlag(seg, "-exec") || containsFlag(seg, "-execdir") ||
			containsFlag(seg, "-ok") || containsFlag(seg, "-okdir")) {
		return fmt.Errorf("command %q is blocked (find execution predicates are not allowed)", truncate(seg, 60))
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
	workspaceDir, err := normalizeWorkspaceDir(e.WorkspaceDir)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	if workspaceDir != "" {
		cmd.Dir = workspaceDir
	}
	cmd.Env = toolCommandEnv(e.KubeconfigPath, e.ExtraEnv, workspaceDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = runProcessGroupCommand(runCtx, cmd)

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

	targetPath, err := e.resolveWorkspaceWritePath(args.Path)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Sprintf("error creating directory %s: %v", dir, err)
	}
	if err := e.validateResolvedWorkspacePath(targetPath); err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	if err := os.WriteFile(targetPath, []byte(args.Content), 0o644); err != nil {
		return fmt.Sprintf("error writing file: %v", err)
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(args.Content), targetPath)
}

func toolCommandEnv(kubeconfigPath string, extraEnv []string, workspaceDir string) []string {
	env := append([]string{}, os.Environ()...)
	env = append(env, extraEnv...)
	if kubeconfigPath != "" {
		env = append(env, "KUBECONFIG="+kubeconfigPath)
	}
	if workspaceDir != "" {
		env = append(env, "INFRA_BENCH_WORKSPACE="+workspaceDir)
	}
	return env
}

func normalizeWorkspaceDir(workspaceDir string) (string, error) {
	if strings.TrimSpace(workspaceDir) == "" {
		return "", nil
	}
	abs, err := filepath.Abs(workspaceDir)
	if err != nil {
		return "", fmt.Errorf("resolve workspace dir: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("workspace dir %s: %w", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace dir %s is not a directory", abs)
	}
	return filepath.Clean(abs), nil
}

func (e *ToolExecutor) resolveWorkspaceWritePath(path string) (string, error) {
	root, err := normalizeWorkspaceDir(e.WorkspaceDir)
	if err != nil {
		return "", err
	}
	if root == "" {
		root, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve current workspace: %w", err)
		}
		root = filepath.Clean(root)
	}

	var target string
	if filepath.IsAbs(path) {
		target = filepath.Clean(path)
	} else {
		target = filepath.Clean(filepath.Join(root, path))
	}
	if !pathWithin(root, target) {
		return "", fmt.Errorf("path %s escapes workspace %s", path, root)
	}
	return target, nil
}

func (e *ToolExecutor) validateResolvedWorkspacePath(targetPath string) error {
	root, err := normalizeWorkspaceDir(e.WorkspaceDir)
	if err != nil {
		return err
	}
	if root == "" {
		root, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("resolve current workspace: %w", err)
		}
		root = filepath.Clean(root)
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve workspace symlinks: %w", err)
	}

	parent := filepath.Dir(targetPath)
	realParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return fmt.Errorf("resolve parent directory symlinks: %w", err)
	}
	if !pathWithin(realRoot, realParent) {
		return fmt.Errorf("path %s escapes workspace %s through symlink", targetPath, root)
	}

	if _, err := os.Lstat(targetPath); err == nil {
		realTarget, evalErr := filepath.EvalSymlinks(targetPath)
		if evalErr != nil {
			return fmt.Errorf("resolve target symlinks: %w", evalErr)
		}
		if !pathWithin(realRoot, realTarget) {
			return fmt.Errorf("path %s escapes workspace %s through symlink", targetPath, root)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect target path: %w", err)
	}
	return nil
}

func pathWithin(root, candidate string) bool {
	root = filepath.Clean(root)
	candidate = filepath.Clean(candidate)
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel))
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
