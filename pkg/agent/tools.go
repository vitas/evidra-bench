package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// BenchTools returns the tool definitions exposed to the LLM.
func BenchTools() []ToolDef {
	return []ToolDef{
		{
			Name:        "run_command",
			Description: "Execute a shell command against the cluster. Use for kubectl, helm, and other CLI tools. The KUBECONFIG environment variable is already set.",
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
			Name:        "evidra_prescribe",
			Description: "Record infrastructure intent BEFORE executing a mutation. Returns a prescription_id to use in evidra_report.",
			Parameters: map[string]any{
				"type":     "object",
				"required": []string{"tool", "operation", "artifact"},
				"properties": map[string]any{
					"tool": map[string]any{
						"type":        "string",
						"description": "Infrastructure tool (kubectl, helm, terraform)",
					},
					"operation": map[string]any{
						"type":        "string",
						"description": "Operation type (apply, delete, patch, upgrade)",
					},
					"artifact": map[string]any{
						"type":        "string",
						"description": "Raw artifact content (YAML manifest, values file, etc.)",
					},
				},
			},
		},
		{
			Name:        "evidra_report",
			Description: "Report the outcome of an infrastructure operation AFTER execution. Use the prescription_id from evidra_prescribe.",
			Parameters: map[string]any{
				"type":     "object",
				"required": []string{"prescription_id", "verdict", "exit_code"},
				"properties": map[string]any{
					"prescription_id": map[string]any{
						"type":        "string",
						"description": "Prescription ID from evidra_prescribe",
					},
					"verdict": map[string]any{
						"type":        "string",
						"enum":        []string{"success", "failure", "error", "declined"},
						"description": "Outcome of the operation",
					},
					"exit_code": map[string]any{
						"type":        "integer",
						"description": "Exit code of the command (0 for success)",
					},
				},
			},
		},
	}
}

// ToolExecutor runs tool calls against the real environment.
type ToolExecutor struct {
	KubeconfigPath string
	EvidencePath   string
	EvidraBin      string
}

// Execute runs a single tool call and returns the result string.
func (e *ToolExecutor) Execute(ctx context.Context, tc ToolCall) string {
	switch tc.Name {
	case "run_command":
		return e.runCommand(ctx, tc.Arguments)
	case "evidra_prescribe":
		return e.evidraPrescribe(ctx, tc.Arguments)
	case "evidra_report":
		return e.evidraReport(ctx, tc.Arguments)
	default:
		return fmt.Sprintf("unknown tool: %s", tc.Name)
	}
}

// allowedCommandPrefixes restricts which commands the LLM can execute.
var allowedCommandPrefixes = []string{
	"kubectl", "helm", "argocd", "kind",
	"cat", "echo", "grep", "head", "tail", "wc",
	"jq", "yq",
}

// validateCommand checks that a command starts with an allowed prefix.
func validateCommand(command string) error {
	trimmed := strings.TrimSpace(command)
	for _, prefix := range allowedCommandPrefixes {
		if trimmed == prefix || strings.HasPrefix(trimmed, prefix+" ") {
			return nil
		}
	}
	return fmt.Errorf("command %q not in allowlist (allowed: %v)", truncate(trimmed, 50), allowedCommandPrefixes)
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

	cmd := exec.CommandContext(ctx, "bash", "-c", args.Command)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+e.KubeconfigPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	result := stdout.String()
	if stderr.Len() > 0 {
		result += "\nSTDERR: " + stderr.String()
	}
	if err != nil {
		result += fmt.Sprintf("\nExit code: %d", exitCode(err))
	}
	return strings.TrimSpace(result)
}

func (e *ToolExecutor) evidraPrescribe(ctx context.Context, argsJSON string) string {
	var args struct {
		Tool      string `json:"tool"`
		Operation string `json:"operation"`
		Artifact  string `json:"artifact"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("error parsing arguments: %v", err)
	}

	bin := e.EvidraBin
	if bin == "" {
		return "error: evidra binary not configured"
	}

	// Write artifact to temp file
	tmpFile, err := os.CreateTemp("", "evidra-artifact-*.yaml")
	if err != nil {
		return fmt.Sprintf("error creating temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString(args.Artifact); err != nil {
		return fmt.Sprintf("error writing artifact: %v", err)
	}
	tmpFile.Close()

	cmdArgs := []string{
		"prescribe",
		"--evidence-dir", e.EvidencePath,
		"--actor", "bench-agent",
		"--tool", args.Tool,
		"--operation", args.Operation,
		"--signing-mode", "optional",
		"-f", tmpFile.Name(),
	}
	cmd := exec.CommandContext(ctx, bin, cmdArgs...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("evidra prescribe failed: %s\n%v", string(out), err)
	}
	return strings.TrimSpace(string(out))
}

func (e *ToolExecutor) evidraReport(ctx context.Context, argsJSON string) string {
	var args struct {
		PrescriptionID string `json:"prescription_id"`
		Verdict        string `json:"verdict"`
		ExitCode       int    `json:"exit_code"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("error parsing arguments: %v", err)
	}

	bin := e.EvidraBin
	if bin == "" {
		return "error: evidra binary not configured"
	}

	cmdArgs := []string{
		"report",
		"--evidence-dir", e.EvidencePath,
		"--actor", "bench-agent",
		"--prescription", args.PrescriptionID,
		"--verdict", args.Verdict,
		"--exit-code", fmt.Sprintf("%d", args.ExitCode),
		"--signing-mode", "optional",
	}
	cmd := exec.CommandContext(ctx, bin, cmdArgs...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("evidra report failed: %s\n%v", string(out), err)
	}
	return strings.TrimSpace(string(out))
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
