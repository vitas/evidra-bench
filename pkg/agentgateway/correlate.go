package agentgateway

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const correlationWindow = 30 * time.Second

// AuditEvent is a narrow Kubernetes audit-log shape used by the controlled demo
// correlator.
type AuditEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	Verb         string    `json:"verb"`
	ResourceKind string    `json:"resource_kind"`
	Namespace    string    `json:"namespace"`
	ResourceName string    `json:"resource_name"`
	ResultCode   int       `json:"result_code"`
}

// CorrelateClusterEffects matches gateway call events to Kubernetes audit
// entries in a controlled benchmark environment.
func CorrelateClusterEffects(events []Event, auditData []byte) ([]Event, []AuditEvent, error) {
	audits, err := parseAuditEvents(auditData)
	if err != nil {
		return nil, nil, err
	}

	orderedEvents := append([]Event(nil), events...)
	sort.SliceStable(orderedEvents, func(i, j int) bool {
		return orderedEvents[i].Timestamp.Before(orderedEvents[j].Timestamp)
	})

	effects := make([]Event, 0, len(audits))
	unmatched := make([]AuditEvent, 0)

	for _, audit := range audits {
		match, ok := findCorrelationMatch(orderedEvents, audit)
		if !ok {
			unmatched = append(unmatched, audit)
			continue
		}

		effects = append(effects, Event{
			Kind:         KindClusterEffect,
			Timestamp:    audit.Timestamp,
			TraceID:      match.TraceID,
			SessionID:    match.SessionID,
			Target:       match.Target,
			Namespace:    audit.Namespace,
			Verb:         audit.Verb,
			Resource:     joinResourceHint(audit.ResourceKind, audit.ResourceName),
			ResourceKind: audit.ResourceKind,
			ResourceName: audit.ResourceName,
			ResultCode:   audit.ResultCode,
		})
	}

	return effects, unmatched, nil
}

func findCorrelationMatch(events []Event, audit AuditEvent) (Event, bool) {
	for _, event := range events {
		if event.Kind != KindGatewayCall {
			continue
		}
		if event.Timestamp.After(audit.Timestamp) {
			continue
		}
		if audit.Timestamp.Sub(event.Timestamp) > correlationWindow {
			continue
		}
		if event.Namespace != "" && audit.Namespace != "" && event.Namespace != audit.Namespace {
			continue
		}
		if event.Resource != "" && !resourceHintMatches(event.Resource, audit.ResourceKind, audit.ResourceName) {
			continue
		}
		return event, true
	}
	return Event{}, false
}

func resourceHintMatches(hint, kind, name string) bool {
	if hint == "" {
		return true
	}
	want := strings.ToLower(joinResourceHint(kind, name))
	hint = strings.ToLower(hint)
	return hint == want || strings.Contains(hint, want)
}

func joinResourceHint(kind, name string) string {
	switch {
	case kind != "" && name != "":
		return kind + "/" + name
	case kind != "":
		return kind
	default:
		return name
	}
}

func parseAuditEvents(data []byte) ([]AuditEvent, error) {
	var raw []struct {
		StageTimestamp string `json:"stageTimestamp"`
		Verb           string `json:"verb"`
		ObjectRef      struct {
			Resource  string `json:"resource"`
			Namespace string `json:"namespace"`
			Name      string `json:"name"`
		} `json:"objectRef"`
		ResponseStatus struct {
			Code int `json:"code"`
		} `json:"responseStatus"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("agentgateway.parseAuditEvents: unmarshal: %w", err)
	}

	audits := make([]AuditEvent, 0, len(raw))
	for _, item := range raw {
		ts, err := time.Parse(time.RFC3339, item.StageTimestamp)
		if err != nil {
			return nil, fmt.Errorf("agentgateway.parseAuditEvents: parse timestamp: %w", err)
		}

		audits = append(audits, AuditEvent{
			Timestamp:    ts,
			Verb:         item.Verb,
			ResourceKind: singularResource(item.ObjectRef.Resource),
			Namespace:    item.ObjectRef.Namespace,
			ResourceName: item.ObjectRef.Name,
			ResultCode:   item.ResponseStatus.Code,
		})
	}

	sort.SliceStable(audits, func(i, j int) bool {
		return audits[i].Timestamp.Before(audits[j].Timestamp)
	})
	return audits, nil
}

func singularResource(value string) string {
	if strings.HasSuffix(value, "ies") {
		return strings.TrimSuffix(value, "ies") + "y"
	}
	if strings.HasSuffix(value, "ses") {
		return strings.TrimSuffix(value, "es")
	}
	if strings.HasSuffix(value, "s") {
		return strings.TrimSuffix(value, "s")
	}
	return value
}
