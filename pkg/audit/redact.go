package audit

import "encoding/json"

// Redacted returns copies of events stripped to metadata: request/response
// bodies and annotations never enter evidence artifacts unless the run
// explicitly opted into body capture (design §5; the mutation-verb policy
// level "Request" can carry secrets — a token pasted into `kubectl annotate`
// or a pod env — and snapshots get the same treatment).
func Redacted(events []Event) []Event {
	out := make([]Event, 0, len(events))
	for _, e := range events {
		c := e
		c.RequestObject = nil
		c.ResponseObject = nil
		c.Annotations = nil
		out = append(out, c)
	}
	return out
}

// RedactJSONL serializes events as newline-delimited JSON post-redaction —
// the only form that may be written to the evidence bundle.
func RedactJSONL(data []byte) ([]byte, error) {
	st := Parse(data)
	out, err := json.Marshal(Redacted(st.Events))
	if err != nil {
		return nil, err
	}
	return out, nil
}
