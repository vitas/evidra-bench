package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// SmartPrescribeTools returns the simplified tool definitions for smart prescribe mode.
// Only 4 fields (tool, operation, resource, namespace) instead of the full schema.
func SmartPrescribeTools() []ToolDef {
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
						"description": "Shell command to execute",
					},
				},
			},
		},
		{
			Name:        "evidra_prescribe",
			Description: "Record intent BEFORE an infrastructure mutation. Call this before every kubectl apply, patch, delete, helm upgrade, etc.",
			Parameters: map[string]any{
				"type":     "object",
				"required": []string{"tool", "operation"},
				"properties": map[string]any{
					"tool": map[string]any{
						"type":        "string",
						"description": "Infrastructure tool (kubectl, helm, terraform)",
					},
					"operation": map[string]any{
						"type":        "string",
						"description": "Operation (apply, delete, patch, upgrade, rollback)",
					},
					"resource": map[string]any{
						"type":        "string",
						"description": "Target resource (e.g. deployment/web, configmap/app-config)",
					},
					"namespace": map[string]any{
						"type":        "string",
						"description": "Kubernetes namespace",
					},
				},
			},
		},
		{
			Name:        "evidra_report",
			Description: "Record outcome AFTER an infrastructure mutation. Call this after every prescribed operation completes or is declined.",
			Parameters: map[string]any{
				"type":     "object",
				"required": []string{"prescription_id", "verdict"},
				"properties": map[string]any{
					"prescription_id": map[string]any{
						"type":        "string",
						"description": "The prescription_id from the prescribe call",
					},
					"verdict": map[string]any{
						"type":        "string",
						"description": "Outcome: success, failure, error, or declined",
						"enum":        []string{"success", "failure", "error", "declined"},
					},
					"exit_code": map[string]any{
						"type":        "integer",
						"description": "Command exit code (0 = success)",
					},
				},
			},
		},
	}
}

// SmartToolExecutor handles tool calls with the simplified prescribe schema.
// It doesn't need the evidra binary — it records evidence directly.
type SmartToolExecutor struct {
	Base     *ToolExecutor // delegates run_command to the base executor
	Evidence *SimpleProxyEvidence
	counter  int
}

// EvidenceMode returns EvidenceModeSmart — simplified prescribe schema.
func (e *SmartToolExecutor) EvidenceMode() EvidenceMode {
	return EvidenceModeSmart
}

// Execute handles run_command, evidra_prescribe (smart), and evidra_report.
func (e *SmartToolExecutor) Execute(ctx context.Context, tc ToolCall) string {
	switch tc.Name {
	case "run_command":
		return e.Base.runCommand(ctx, tc.Arguments)

	case "evidra_prescribe":
		return e.smartPrescribe(tc.Arguments)

	case "evidra_report":
		return e.smartReport(tc.Arguments)

	default:
		return fmt.Sprintf("unknown tool: %s", tc.Name)
	}
}

func (e *SmartToolExecutor) smartPrescribe(argsJSON string) string {
	var args struct {
		Tool      string `json:"tool"`
		Operation string `json:"operation"`
		Resource  string `json:"resource"`
		Namespace string `json:"namespace"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("error parsing arguments: %v", err)
	}
	if args.Tool == "" || args.Operation == "" {
		return "error: tool and operation are required"
	}

	e.counter++
	id := fmt.Sprintf("smart-%d-%d", time.Now().UnixMilli(), e.counter)

	if e.Evidence != nil {
		cmd := fmt.Sprintf("%s %s %s -n %s", args.Tool, args.Operation, args.Resource, args.Namespace)
		e.Evidence.Prescribe(cmd)
	}

	log.Printf("[smart-prescribe] %s %s %s/%s → %s", args.Tool, args.Operation, args.Namespace, args.Resource, id)

	result := map[string]any{
		"ok":              true,
		"prescription_id": id,
	}
	data, _ := json.Marshal(result)
	return string(data)
}

func (e *SmartToolExecutor) smartReport(argsJSON string) string {
	var args struct {
		PrescriptionID string `json:"prescription_id"`
		Verdict        string `json:"verdict"`
		ExitCode       *int   `json:"exit_code"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("error parsing arguments: %v", err)
	}
	if args.PrescriptionID == "" || args.Verdict == "" {
		return "error: prescription_id and verdict are required"
	}

	exitCode := 0
	if args.ExitCode != nil {
		exitCode = *args.ExitCode
	}

	if e.Evidence != nil {
		e.Evidence.Report(args.PrescriptionID, exitCode)
	}

	log.Printf("[smart-report] %s verdict=%s exit=%d", args.PrescriptionID, args.Verdict, exitCode)

	result := map[string]any{
		"ok":        true,
		"report_id": fmt.Sprintf("rep-%d", time.Now().UnixMilli()),
	}
	data, _ := json.Marshal(result)
	return string(data)
}
