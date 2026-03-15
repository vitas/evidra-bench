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

	"samebits.com/evidra/pkg/execcontract"
)

// BenchTools returns the tool definitions exposed to the LLM.
func BenchTools() []ToolDef {
	prescribeDef, reportDef := mustEvidraToolDefinitions()

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
			Name:        prescribeDef.Name,
			Description: prescribeDef.Description,
			Parameters:  prescribeDef.Parameters,
		},
		{
			Name:        reportDef.Name,
			Description: reportDef.Description,
			Parameters:  reportDef.Parameters,
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
	var input execcontract.PrescribeInput
	if err := json.Unmarshal([]byte(argsJSON), &input); err != nil {
		return fmt.Sprintf("error parsing arguments: %v", err)
	}
	if err := execcontract.ValidatePrescribeInput(input); err != nil {
		return fmt.Sprintf("error validating arguments: %v", err)
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
	if _, err := tmpFile.WriteString(input.RawArtifact); err != nil {
		return fmt.Sprintf("error writing artifact: %v", err)
	}
	tmpFile.Close()

	cmdArgs, err := buildPrescribeCommandArgs(e.EvidencePath, tmpFile.Name(), input)
	if err != nil {
		return fmt.Sprintf("error building prescribe command: %v", err)
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
	var input execcontract.ReportInput
	if err := json.Unmarshal([]byte(argsJSON), &input); err != nil {
		return fmt.Sprintf("error parsing arguments: %v", err)
	}
	if err := execcontract.ValidateReportInput(input); err != nil {
		return fmt.Sprintf("error validating arguments: %v", err)
	}

	bin := e.EvidraBin
	if bin == "" {
		return "error: evidra binary not configured"
	}

	cmdArgs, err := buildReportCommandArgs(e.EvidencePath, input)
	if err != nil {
		return fmt.Sprintf("error building report command: %v", err)
	}
	cmd := exec.CommandContext(ctx, bin, cmdArgs...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("evidra report failed: %s\n%v", string(out), err)
	}
	return strings.TrimSpace(string(out))
}

func mustEvidraToolDefinitions() (execcontract.ToolDefinition, execcontract.ToolDefinition) {
	prescribeDef, err := execcontract.PrescribeToolDefinition()
	if err != nil {
		panic(fmt.Sprintf("agent.BenchTools: load prescribe tool definition: %v", err))
	}
	reportDef, err := execcontract.ReportToolDefinition()
	if err != nil {
		panic(fmt.Sprintf("agent.BenchTools: load report tool definition: %v", err))
	}
	return prescribeDef, reportDef
}

func buildPrescribeCommandArgs(evidencePath, artifactPath string, input execcontract.PrescribeInput) ([]string, error) {
	if err := execcontract.ValidatePrescribeInput(input); err != nil {
		return nil, err
	}
	args := []string{
		"prescribe",
		"--evidence-dir", evidencePath,
		"--tool", input.Tool,
		"--operation", input.Operation,
		"--signing-mode", "optional",
		"-f", artifactPath,
	}
	args = appendActorArgs(args, input.Actor)
	if input.Environment != "" {
		args = append(args, "--environment", input.Environment)
	}
	if input.SessionID != "" {
		args = append(args, "--session-id", input.SessionID)
	}
	if input.OperationID != "" {
		args = append(args, "--operation-id", input.OperationID)
	}
	if input.Attempt > 0 {
		args = append(args, "--attempt", fmt.Sprintf("%d", input.Attempt))
	}
	if input.TraceID != "" {
		args = append(args, "--trace-id", input.TraceID)
	}
	if input.SpanID != "" {
		args = append(args, "--span-id", input.SpanID)
	}
	if input.ParentSpanID != "" {
		args = append(args, "--parent-span-id", input.ParentSpanID)
	}
	if len(input.ScopeDimensions) > 0 {
		raw, err := json.Marshal(input.ScopeDimensions)
		if err != nil {
			return nil, fmt.Errorf("marshal scope_dimensions: %w", err)
		}
		args = append(args, "--scope-dimensions", string(raw))
	}
	if input.CanonicalAction != nil {
		raw, err := json.Marshal(input.CanonicalAction)
		if err != nil {
			return nil, fmt.Errorf("marshal canonical_action: %w", err)
		}
		args = append(args, "--canonical-action", string(raw))
	}
	return args, nil
}

func buildReportCommandArgs(evidencePath string, input execcontract.ReportInput) ([]string, error) {
	if err := execcontract.ValidateReportInput(input); err != nil {
		return nil, err
	}
	args := []string{
		"report",
		"--evidence-dir", evidencePath,
		"--prescription", input.PrescriptionID,
		"--verdict", input.Verdict,
		"--signing-mode", "optional",
	}
	args = appendActorArgs(args, input.Actor)
	if input.ExitCode != nil {
		args = append(args, "--exit-code", fmt.Sprintf("%d", *input.ExitCode))
	}
	if input.DecisionContext != nil {
		args = append(args, "--decline-trigger", input.DecisionContext.Trigger)
		args = append(args, "--decline-reason", input.DecisionContext.Reason)
	}
	if input.ArtifactDigest != "" {
		args = append(args, "--artifact-digest", input.ArtifactDigest)
	}
	if input.SessionID != "" {
		args = append(args, "--session-id", input.SessionID)
	}
	if input.OperationID != "" {
		args = append(args, "--operation-id", input.OperationID)
	}
	if input.SpanID != "" {
		args = append(args, "--span-id", input.SpanID)
	}
	if input.ParentSpanID != "" {
		args = append(args, "--parent-span-id", input.ParentSpanID)
	}
	if len(input.ExternalRefs) > 0 {
		raw, err := json.Marshal(input.ExternalRefs)
		if err != nil {
			return nil, fmt.Errorf("marshal external_refs: %w", err)
		}
		args = append(args, "--external-refs", string(raw))
	}
	return args, nil
}

func appendActorArgs(args []string, actor execcontract.Actor) []string {
	if actor.ID != "" {
		args = append(args, "--actor", actor.ID)
	}
	if actor.Type != "" {
		args = append(args, "--actor-type", actor.Type)
	}
	if actor.Origin != "" {
		args = append(args, "--actor-origin", actor.Origin)
	}
	if actor.InstanceID != "" {
		args = append(args, "--actor-instance-id", actor.InstanceID)
	}
	if actor.Version != "" {
		args = append(args, "--actor-version", actor.Version)
	}
	if actor.SkillVersion != "" {
		args = append(args, "--actor-skill-version", actor.SkillVersion)
	}
	return args
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
