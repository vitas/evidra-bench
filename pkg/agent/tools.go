package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
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

// ProxyEvidenceWriter records proxy-mode evidence (auto prescribe/report).
// Implemented by proxy.EvidenceWriter from the parent evidra project.
// When nil, proxy mode is disabled.
type ProxyEvidenceWriter interface {
	Prescribe(command string) string
	Report(prescriptionID string, exitCode int)
}

// ToolExecutor runs tool calls against the real environment.
//
// When ProxyEvidence is set, the executor auto-records prescribe/report
// evidence for mutation commands (kubectl apply, helm upgrade, etc.)
// without any agent involvement — zero extra tokens. This is "proxy mode"
// as opposed to "direct mode" where the agent calls evidra_prescribe/report
// explicitly.
type ToolExecutor struct {
	KubeconfigPath string
	EvidencePath   string
	EvidraBin      string
	ProxyEvidence  ProxyEvidenceWriter // nil = proxy mode disabled
}

// EvidenceMode returns how this executor records evidence.
func (e *ToolExecutor) EvidenceMode() EvidenceMode {
	if e.ProxyEvidence != nil {
		return EvidenceModeProxy
	}
	if e.EvidraBin != "" {
		return EvidenceModeDirect
	}
	return EvidenceModeNone
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
	"kubectl", "helm", "argocd", "kind", "terraform",
	"cat", "echo", "grep", "head", "tail", "wc", "ls", "find",
	"jq", "yq",
}

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
func validateCommand(command string) error {
	trimmed := strings.TrimSpace(command)

	// Check blocklist first.
	for _, blocked := range blockedSubcommands {
		if trimmed == strings.TrimSpace(blocked) || strings.HasPrefix(trimmed, blocked) {
			return fmt.Errorf("command %q is blocked (interactive/dangerous)", truncate(trimmed, 60))
		}
	}

	// Check allowlist.
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

	// Proxy mode: auto-prescribe before mutations, auto-report after.
	var prescriptionID string
	if e.ProxyEvidence != nil && isMutationCommand(args.Command) {
		prescriptionID = e.ProxyEvidence.Prescribe(args.Command)
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
	ec := 0
	if err != nil {
		ec = exitCode(err)
		result += fmt.Sprintf("\nExit code: %d", ec)
	}

	// Proxy mode: auto-report after execution.
	if prescriptionID != "" {
		e.ProxyEvidence.Report(prescriptionID, ec)
	}

	return strings.TrimSpace(result)
}

// mutationSubcommands maps infrastructure tools to their mutating subcommands.
// Mirrors pkg/proxy/detect.go from the parent evidra project.
var mutationSubcommands = map[string]map[string]bool{
	"kubectl": {"apply": true, "create": true, "patch": true, "replace": true, "delete": true,
		"set": true, "annotate": true, "label": true, "rollout": true, "scale": true,
		"taint": true, "cordon": true, "uncordon": true, "drain": true},
	"helm":      {"install": true, "upgrade": true, "uninstall": true, "rollback": true},
	"terraform": {"apply": true, "destroy": true, "import": true},
	"argocd":    {"sync": true, "delete": true},
}

// isMutationCommand returns true if the command modifies infrastructure state.
func isMutationCommand(command string) bool {
	words := strings.Fields(strings.TrimSpace(command))
	if len(words) < 2 {
		return false
	}
	subs, ok := mutationSubcommands[words[0]]
	if !ok {
		return false
	}
	return subs[words[1]]
}

func (e *ToolExecutor) evidraPrescribe(ctx context.Context, argsJSON string) string {
	argsJSON = fixStringifiedJSONFields(argsJSON, "actor", "canonical_action", "scope_dimensions")
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
	argsJSON = fixStringifiedJSONFields(argsJSON, "actor", "decision_context", "external_refs")
	argsJSON = fixStringifiedIntFields(argsJSON, "exit_code")
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
	// Sanitize schemas for strict providers (OpenAI requires "items" on arrays).
	sanitizeArrayItems(prescribeDef.Parameters)
	sanitizeArrayItems(reportDef.Parameters)
	return prescribeDef, reportDef
}

// sanitizeArrayItems walks a JSON Schema map and adds "items": {} to any
// array-typed property that lacks it. OpenAI rejects tool schemas without items.
func sanitizeArrayItems(schema map[string]any) {
	typ, _ := schema["type"].(string)
	if typ == "array" {
		if _, ok := schema["items"]; !ok {
			schema["items"] = map[string]any{}
		}
	}
	if props, ok := schema["properties"].(map[string]any); ok {
		for _, v := range props {
			if sub, ok := v.(map[string]any); ok {
				sanitizeArrayItems(sub)
			}
		}
	}
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

// fixStringifiedJSONFields handles the case where Claude sends nested objects
// as JSON strings instead of objects. For example:
//
//	"actor": "{\"type\":\"agent\"}" → "actor": {"type":"agent"}
func fixStringifiedJSONFields(argsJSON string, fields ...string) string {
	var raw map[string]json.RawMessage
	if json.Unmarshal([]byte(argsJSON), &raw) != nil {
		return argsJSON
	}
	changed := false
	for _, field := range fields {
		val, ok := raw[field]
		if !ok {
			continue
		}
		// Check if the value is a string (starts with '"')
		trimmed := bytes.TrimSpace(val)
		if len(trimmed) > 0 && trimmed[0] == '"' {
			var s string
			if json.Unmarshal(trimmed, &s) == nil {
				// Try to parse the string as JSON
				var parsed json.RawMessage
				if json.Unmarshal([]byte(s), &parsed) == nil {
					raw[field] = parsed
					changed = true
				}
			}
		}
	}
	if !changed {
		return argsJSON
	}
	fixed, err := json.Marshal(raw)
	if err != nil {
		return argsJSON
	}
	return string(fixed)
}

// fixStringifiedIntFields converts string integers to actual integers.
// Handles: "exit_code": "0" → "exit_code": 0
func fixStringifiedIntFields(argsJSON string, fields ...string) string {
	var raw map[string]json.RawMessage
	if json.Unmarshal([]byte(argsJSON), &raw) != nil {
		return argsJSON
	}
	changed := false
	for _, field := range fields {
		val, ok := raw[field]
		if !ok {
			continue
		}
		trimmed := bytes.TrimSpace(val)
		if len(trimmed) > 0 && trimmed[0] == '"' {
			var s string
			if json.Unmarshal(trimmed, &s) == nil {
				if _, err := strconv.Atoi(s); err == nil {
					raw[field] = json.RawMessage(s)
					changed = true
				}
			}
		}
	}
	if !changed {
		return argsJSON
	}
	fixed, err := json.Marshal(raw)
	if err != nil {
		return argsJSON
	}
	return string(fixed)
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
