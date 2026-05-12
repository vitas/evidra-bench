package bench

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// mutationSubcommands maps infrastructure tools to their mutating subcommands.
var mutationSubcommands = map[string]map[string]bool{
	"kubectl": {
		"apply": true, "create": true, "patch": true, "replace": true, "delete": true,
		"set": true, "annotate": true, "label": true, "rollout": true, "scale": true,
		"taint": true, "cordon": true, "uncordon": true, "drain": true,
	},
	"helm": {
		"install": true, "upgrade": true, "uninstall": true, "rollback": true,
	},
	"terraform": {
		"apply": true, "destroy": true, "import": true,
	},
	"argocd": {
		"sync": true, "delete": true,
	},
}

// diagnosisCommands are subcommands that always indicate diagnosis.
var diagnosisCommands = map[string]bool{
	"describe": true,
	"logs":     true,
	"events":   true,
}

// Parse classifies a sequence of tool calls into a decision timeline.
func Parse(calls []ToolCall) *Timeline {
	tl := &Timeline{
		Steps:      make([]TimelineStep, 0, len(calls)),
		PhaseCount: make(map[Phase]int),
	}
	if len(calls) == 0 {
		return tl
	}

	seenMutation := false
	readOnlyCount := 0

	for i, call := range calls {
		step := TimelineStep{
			Index: i,
			Tool:  call.Tool,
		}

		switch call.Tool {
		case "run_command":
			cmd := extractCommand(call.Args)
			step.Command = cmd
			infraTool, subcommand, rest := parseCommand(cmd)
			step.Operation = subcommand

			ns := extractNamespace(cmd)
			step.Namespace = ns
			step.Resource = extractResource(rest, subcommand)
			step.ExitCode = extractExitCode(call.Result)

			if isMutationCmd(infraTool, subcommand, cmd) {
				step.Phase = PhaseAct
				seenMutation = true
				tl.MutationCount++
				step.Summary = buildMutationSummary(subcommand, step.Resource, ns)
			} else {
				if seenMutation {
					step.Phase = PhaseVerify
				} else if isDiagnosis(infraTool, subcommand, cmd) || readOnlyCount > 0 {
					step.Phase = PhaseDiagnose
					tl.DiagnosisDepth++
				} else {
					step.Phase = PhaseDiscover
				}
				readOnlyCount++
				step.Summary = buildReadSummary(subcommand, step.Resource, ns)
			}

		default:
			var ok bool
			step, ok = parseMCPToolCall(i, call, seenMutation, readOnlyCount)
			if !ok {
				// Unknown non-run_command tools are skipped.
				continue
			}
			if step.Phase == PhaseAct {
				seenMutation = true
				tl.MutationCount++
			} else {
				readOnlyCount++
			}
		}

		tl.Steps = append(tl.Steps, step)
		tl.PhaseCount[step.Phase]++
	}

	tl.TotalSteps = len(tl.Steps)
	return tl
}

func parseMCPToolCall(index int, call ToolCall, seenMutation bool, readOnlyCount int) (TimelineStep, bool) {
	step := TimelineStep{
		Index: index,
		Tool:  call.Tool,
	}

	switch call.Tool {
	case "resources_create_or_update":
		ref := resourceRefFromManifestArg(call.Args)
		step.Operation = "create_or_update"
		step.Resource = ref.String()
		step.Namespace = ref.Namespace
		step.Command = formatMCPCommand(call.Tool, step.Resource, step.Namespace)
		step.ExitCode = extractExitCode(call.Result)
		step.Phase = PhaseAct
		step.Summary = buildMutationSummary(step.Operation, step.Resource, step.Namespace)
		return step, true
	case "resources_delete":
		ref := resourceRefFromArgs(call.Args)
		step.Operation = "delete"
		step.Resource = ref.String()
		step.Namespace = ref.Namespace
		step.Command = formatMCPCommand(call.Tool, step.Resource, step.Namespace)
		step.ExitCode = extractExitCode(call.Result)
		step.Phase = PhaseAct
		step.Summary = buildMutationSummary(step.Operation, step.Resource, step.Namespace)
		return step, true
	case "pods_delete":
		ref := resourceRefFromArgs(call.Args)
		ref.Kind = "Pod"
		step.Operation = "delete"
		step.Resource = ref.String()
		step.Namespace = ref.Namespace
		step.Command = formatMCPCommand(call.Tool, step.Resource, step.Namespace)
		step.ExitCode = extractExitCode(call.Result)
		step.Phase = PhaseAct
		step.Summary = buildMutationSummary(step.Operation, step.Resource, step.Namespace)
		return step, true
	case "resources_scale":
		ref := resourceRefFromArgs(call.Args)
		step.Operation = "scale"
		step.Resource = ref.String()
		step.Namespace = ref.Namespace
		step.Command = formatMCPCommand(call.Tool, step.Resource, step.Namespace)
		step.ExitCode = extractExitCode(call.Result)
		step.Phase = PhaseAct
		step.Summary = buildMutationSummary(step.Operation, step.Resource, step.Namespace)
		return step, true
	case "resources_get", "resources_list", "pods_get", "pods_list", "pods_list_in_namespace", "pods_log", "pods_exec", "events_list":
		ref := resourceRefFromArgs(call.Args)
		if call.Tool == "pods_get" || call.Tool == "pods_log" || call.Tool == "pods_exec" {
			ref.Kind = "Pod"
		}
		step.Operation = mcpReadOperation(call.Tool)
		step.Resource = ref.String()
		step.Namespace = ref.Namespace
		step.Command = formatMCPCommand(call.Tool, step.Resource, step.Namespace)
		step.ExitCode = extractExitCode(call.Result)
		if seenMutation {
			step.Phase = PhaseVerify
		} else if readOnlyCount > 0 || isMCPDiagnosticTool(call.Tool) {
			step.Phase = PhaseDiagnose
		} else {
			step.Phase = PhaseDiscover
		}
		step.Summary = buildReadSummary(step.Operation, step.Resource, step.Namespace)
		return step, true
	default:
		return TimelineStep{}, false
	}
}

type kubernetesResourceRef struct {
	Kind      string
	Name      string
	Namespace string
}

func (r kubernetesResourceRef) String() string {
	if r.Kind == "" && r.Name == "" {
		return ""
	}
	if r.Kind == "" {
		return r.Name
	}
	if r.Name == "" {
		return r.Kind
	}
	return r.Kind + "/" + r.Name
}

func resourceRefFromArgs(args json.RawMessage) kubernetesResourceRef {
	var parsed struct {
		Kind      string `json:"kind"`
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	}
	_ = json.Unmarshal(args, &parsed)
	return kubernetesResourceRef{
		Kind:      strings.TrimSpace(parsed.Kind),
		Name:      strings.TrimSpace(parsed.Name),
		Namespace: strings.TrimSpace(parsed.Namespace),
	}
}

func resourceRefFromManifestArg(args json.RawMessage) kubernetesResourceRef {
	var parsed struct {
		Resource string `json:"resource"`
	}
	if err := json.Unmarshal(args, &parsed); err != nil {
		return kubernetesResourceRef{}
	}
	return resourceRefFromManifest(parsed.Resource)
}

func resourceRefFromManifest(manifest string) kubernetesResourceRef {
	var parsed struct {
		Kind     string `yaml:"kind"`
		Metadata struct {
			Name      string `yaml:"name"`
			Namespace string `yaml:"namespace"`
		} `yaml:"metadata"`
	}
	if err := yaml.Unmarshal([]byte(manifest), &parsed); err != nil {
		return kubernetesResourceRef{}
	}
	return kubernetesResourceRef{
		Kind:      strings.TrimSpace(parsed.Kind),
		Name:      strings.TrimSpace(parsed.Metadata.Name),
		Namespace: strings.TrimSpace(parsed.Metadata.Namespace),
	}
}

func formatMCPCommand(tool, resource, namespace string) string {
	parts := []string{tool}
	if resource != "" {
		parts = append(parts, resource)
	}
	if namespace != "" {
		parts = append(parts, "in", namespace)
	}
	return strings.Join(parts, " ")
}

func mcpReadOperation(tool string) string {
	switch tool {
	case "pods_log":
		return "logs"
	case "pods_exec":
		return "exec"
	case "events_list":
		return "events"
	default:
		return "get"
	}
}

func isMCPDiagnosticTool(tool string) bool {
	switch tool {
	case "pods_log", "pods_exec", "events_list", "resources_get", "pods_get":
		return true
	default:
		return false
	}
}

// extractCommand pulls the command string from tool call args JSON.
func extractCommand(args json.RawMessage) string {
	var parsed struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(args, &parsed); err != nil {
		return ""
	}
	return parsed.Command
}

// parseCommand splits a shell command into infrastructure tool, subcommand, and the rest.
func parseCommand(cmd string) (infraTool, subcommand, rest string) {
	words := strings.Fields(cmd)
	if len(words) < 1 {
		return "", "", ""
	}
	infraTool = words[0]
	if len(words) >= 2 {
		subcommand = words[1]
	}
	if len(words) >= 3 {
		rest = strings.Join(words[2:], " ")
	}
	return infraTool, subcommand, rest
}

// readOnlyRollout contains kubectl rollout subcommands that are read-only.
var readOnlyRollout = map[string]bool{
	"status":  true,
	"history": true,
}

// isMutationCmd returns true if the command is a mutating infrastructure command.
// It needs the full command to distinguish e.g. "rollout status" (read) from "rollout restart" (write).
func isMutationCmd(infraTool, subcommand, fullCmd string) bool {
	subs, ok := mutationSubcommands[infraTool]
	if !ok {
		return false
	}
	if !subs[subcommand] {
		return false
	}
	// kubectl rollout status/history are read-only.
	if infraTool == "kubectl" && subcommand == "rollout" {
		words := strings.Fields(fullCmd)
		if len(words) >= 3 && readOnlyRollout[words[2]] {
			return false
		}
	}
	return true
}

// isDiagnosis returns true if the read-only command indicates diagnosis (not discovery).
func isDiagnosis(infraTool, subcommand, fullCmd string) bool {
	if infraTool != "kubectl" {
		return false
	}
	if diagnosisCommands[subcommand] {
		return true
	}
	// kubectl get with -o yaml/json is diagnosis, not discovery.
	if subcommand == "get" {
		if strings.Contains(fullCmd, " -o yaml") || strings.Contains(fullCmd, " -o json") ||
			strings.Contains(fullCmd, " -oyaml") || strings.Contains(fullCmd, " -ojson") {
			return true
		}
		// kubectl get events is diagnosis.
		if strings.Contains(fullCmd, " events") {
			return true
		}
	}
	return false
}

// extractNamespace parses -n <namespace> from a command string.
func extractNamespace(cmd string) string {
	words := strings.Fields(cmd)
	for i, w := range words {
		if (w == "-n" || w == "--namespace") && i+1 < len(words) {
			return words[i+1]
		}
	}
	return ""
}

// extractResource finds the resource argument from the remaining command words.
// It looks for patterns like "deployment/web", "pod/foo", or "deployment web".
func extractResource(rest, subcommand string) string {
	if rest == "" {
		return ""
	}
	words := strings.Fields(rest)
	for _, w := range words {
		if strings.HasPrefix(w, "-") {
			continue
		}
		// resource/name pattern (e.g., "deployment/web").
		if strings.Contains(w, "/") {
			return w
		}
		// First non-flag word is the resource type or name.
		return w
	}
	return ""
}

// extractExitCode checks for exit code patterns in the result string.
// Tool call results from the harness don't include exit codes in a structured way,
// so we default to 0 for non-empty results that don't contain error indicators.
func extractExitCode(result string) int {
	if strings.Contains(result, "exit status 1") || strings.Contains(result, "exit code 1") {
		return 1
	}
	if strings.HasPrefix(result, "error:") || strings.HasPrefix(result, "Error") {
		return 1
	}
	return 0
}

// buildMutationSummary generates a human-readable summary for a mutating command.
func buildMutationSummary(subcommand, resource, ns string) string {
	var verb string
	switch subcommand {
	case "apply":
		verb = "Applied"
	case "create_or_update":
		verb = "Applied"
	case "create":
		verb = "Created"
	case "patch":
		verb = "Patched"
	case "delete":
		verb = "Deleted"
	case "replace":
		verb = "Replaced"
	case "scale":
		verb = "Scaled"
	case "rollout":
		verb = "Rolled out"
	case "label":
		verb = "Labeled"
	case "annotate":
		verb = "Annotated"
	case "drain":
		verb = "Drained"
	case "cordon":
		verb = "Cordoned"
	case "uncordon":
		verb = "Uncordoned"
	case "taint":
		verb = "Tainted"
	case "install":
		verb = "Installed"
	case "upgrade":
		verb = "Upgraded"
	case "uninstall":
		verb = "Uninstalled"
	case "rollback":
		verb = "Rolled back"
	default:
		verb = "Ran " + subcommand
	}
	return formatSummary(verb, resource, ns)
}

// buildReadSummary generates a human-readable summary for a read-only command.
func buildReadSummary(subcommand, resource, ns string) string {
	var verb string
	switch subcommand {
	case "get":
		verb = "Listed"
	case "describe":
		verb = "Described"
	case "logs":
		verb = "Checked logs for"
	case "rollout":
		verb = "Checked rollout of"
	default:
		verb = "Ran " + subcommand + " on"
	}
	return formatSummary(verb, resource, ns)
}

// formatSummary builds the final "Verb resource in namespace" string.
func formatSummary(verb, resource, ns string) string {
	if resource == "" && ns == "" {
		return verb
	}
	if resource == "" {
		return fmt.Sprintf("%s in %s", verb, ns)
	}
	if ns == "" {
		return fmt.Sprintf("%s %s", verb, resource)
	}
	return fmt.Sprintf("%s %s in %s", verb, resource, ns)
}
