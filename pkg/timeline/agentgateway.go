package timeline

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"samebits.com/evidra-infra-bench/pkg/agentgateway"
)

// ParseAgentGateway converts normalized AgentGateway events into the existing
// timeline model.
func ParseAgentGateway(events []agentgateway.Event) *Timeline {
	tl := newTimeline(len(events))
	if len(events) == 0 {
		return tl
	}

	ordered := append([]agentgateway.Event(nil), events...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Timestamp.Before(ordered[j].Timestamp)
	})

	for _, event := range ordered {
		step := TimelineStep{
			Index:     len(tl.Steps),
			Kind:      string(event.Kind),
			Timestamp: event.Timestamp.Format(time.RFC3339),
			Tool:      event.Tool,
			Operation: event.Method,
			SessionID: event.SessionID,
			Target:    event.Target,
			Namespace: event.Namespace,
			Resource:  event.Resource,
		}

		switch event.Kind {
		case agentgateway.KindPolicyDecision:
			step.Phase = PhaseDecide
			step.Operation = string(event.Decision)
			step.Summary = buildPolicySummary(event)
		case agentgateway.KindGatewayCall:
			step.Phase = PhaseAct
			step.Summary = buildGatewaySummary(event)
			tl.MutationCount++
		case agentgateway.KindClusterEffect:
			step.Phase = PhaseVerify
			step.Operation = event.Verb
			step.Resource = joinResource(event.ResourceKind, event.ResourceName, event.Resource)
			step.Summary = buildClusterEffectSummary(event)
		default:
			continue
		}

		tl.Steps = append(tl.Steps, step)
		tl.PhaseCount[step.Phase]++
	}

	tl.TotalSteps = len(tl.Steps)
	return tl
}

func buildPolicySummary(event agentgateway.Event) string {
	verb := "Allowed"
	if event.Decision == agentgateway.DecisionDeny {
		verb = "Denied"
	}

	switch {
	case event.PolicyID != "" && event.PolicyReason != "":
		return fmt.Sprintf("%s by %s: %s", verb, event.PolicyID, event.PolicyReason)
	case event.PolicyID != "":
		return fmt.Sprintf("%s by %s", verb, event.PolicyID)
	case event.PolicyReason != "":
		return fmt.Sprintf("%s: %s", verb, event.PolicyReason)
	default:
		return verb
	}
}

func buildGatewaySummary(event agentgateway.Event) string {
	parts := []string{"Gateway call"}
	if event.Tool != "" {
		parts = append(parts, event.Tool)
	}
	if event.Target != "" {
		parts = append(parts, "on", event.Target)
	}
	summary := strings.Join(parts, " ")
	if event.Status > 0 {
		summary = fmt.Sprintf("%s (status %d)", summary, event.Status)
	}
	return summary
}

func buildClusterEffectSummary(event agentgateway.Event) string {
	resource := joinResource(event.ResourceKind, event.ResourceName, event.Resource)
	verb := strings.ToLower(event.Verb)
	switch verb {
	case "apply":
		return formatSummary("Applied", resource, event.Namespace)
	case "create":
		return formatSummary("Created", resource, event.Namespace)
	case "patch":
		return formatSummary("Patched", resource, event.Namespace)
	case "replace":
		return formatSummary("Replaced", resource, event.Namespace)
	case "delete":
		return formatSummary("Deleted", resource, event.Namespace)
	default:
		if verb == "" {
			return formatSummary("Observed", resource, event.Namespace)
		}
		return formatSummary(titleWord(verb), resource, event.Namespace)
	}
}

func joinResource(kind, name, fallback string) string {
	switch {
	case kind != "" && name != "":
		return kind + "/" + name
	case fallback != "":
		return fallback
	case kind != "":
		return kind
	default:
		return name
	}
}

func titleWord(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
