package agentgateway

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// ParseOTELRecord converts a JSON log/OTEL record into a normalized gateway
// call event.
func ParseOTELRecord(data []byte) (Event, error) {
	record, err := parseRecord(data)
	if err != nil {
		return Event{}, err
	}

	method := stringAttr(record.Attributes, "mcp.method.name")
	if method == "" {
		return Event{}, fmt.Errorf("agentgateway.ParseOTELRecord: missing mcp.method.name")
	}

	return Event{
		Kind:      KindGatewayCall,
		Timestamp: record.Timestamp,
		TraceID:   stringAttr(record.Attributes, "trace.id"),
		SessionID: stringAttr(record.Attributes, "mcp.session.id"),
		Target:    stringAttr(record.Attributes, "mcp.target"),
		Method:    method,
		Tool:      stringAttr(record.Attributes, "gen_ai.tool.name"),
		Status:    intAttr(record.Attributes, "http.status"),
		Reason:    stringAttr(record.Attributes, "reason"),
	}, nil
}

type rawRecord struct {
	Timestamp  time.Time      `json:"-"`
	Attributes map[string]any `json:"-"`
}

func parseRecord(data []byte) (rawRecord, error) {
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		return rawRecord{}, fmt.Errorf("agentgateway.parseRecord: unmarshal: %w", err)
	}

	tsRaw, ok := body["timestamp"]
	if !ok {
		return rawRecord{}, fmt.Errorf("agentgateway.parseRecord: missing timestamp")
	}
	ts, err := parseTimestamp(tsRaw)
	if err != nil {
		return rawRecord{}, err
	}

	attrs := attributesFromBody(body)
	return rawRecord{
		Timestamp:  ts,
		Attributes: attrs,
	}, nil
}

func attributesFromBody(body map[string]any) map[string]any {
	if raw, ok := body["attributes"].(map[string]any); ok {
		return raw
	}

	attrs := make(map[string]any, len(body))
	for key, value := range body {
		if key == "timestamp" {
			continue
		}
		attrs[key] = value
	}
	return attrs
}

func parseTimestamp(raw any) (time.Time, error) {
	s, ok := raw.(string)
	if !ok || s == "" {
		return time.Time{}, fmt.Errorf("agentgateway.parseRecord: invalid timestamp")
	}
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("agentgateway.parseRecord: parse timestamp: %w", err)
	}
	return ts, nil
}

func stringAttr(attrs map[string]any, key string) string {
	raw, ok := attrs[key]
	if !ok {
		return ""
	}
	switch value := raw.(type) {
	case string:
		return value
	case json.Number:
		return value.String()
	default:
		return fmt.Sprint(value)
	}
}

func intAttr(attrs map[string]any, key string) int {
	raw, ok := attrs[key]
	if !ok {
		return 0
	}
	switch value := raw.(type) {
	case float64:
		return int(value)
	case int:
		return value
	case int64:
		return int(value)
	case json.Number:
		n, err := value.Int64()
		if err == nil {
			return int(n)
		}
	case string:
		n, err := strconv.Atoi(value)
		if err == nil {
			return n
		}
	}
	return 0
}
