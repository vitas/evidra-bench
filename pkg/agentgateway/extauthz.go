package agentgateway

import "fmt"

// ParseExtAuthzDecision converts a JSON record carrying extAuthz metadata into
// a normalized policy-decision event.
func ParseExtAuthzDecision(data []byte) (Event, error) {
	record, err := parseRecord(data)
	if err != nil {
		return Event{}, err
	}

	decision := Decision(stringAttr(record.Attributes, "extauthz.decision"))
	if decision != DecisionAllow && decision != DecisionDeny {
		return Event{}, fmt.Errorf("agentgateway.ParseExtAuthzDecision: missing extauthz.decision")
	}

	return Event{
		Kind:         KindPolicyDecision,
		Timestamp:    record.Timestamp,
		TraceID:      stringAttr(record.Attributes, "trace.id"),
		SessionID:    stringAttr(record.Attributes, "mcp.session.id"),
		Decision:     decision,
		PolicyID:     stringAttr(record.Attributes, "extauthz.policy_id"),
		PolicyReason: stringAttr(record.Attributes, "extauthz.reason"),
	}, nil
}
