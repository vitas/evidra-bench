// Package audit collects and evaluates Kubernetes API audit evidence for
// safety qualification (docs/adr/0001-process-safety-matching.md).
//
// The collector is deliberately read-only over captured log text: parsing,
// de-duplication, marker-windowing, and redaction happen in-process, and
// every degradation is surfaced as a coverage state — evidence is never
// quietly downgraded to "looks complete".
package audit

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Legal audit stages (docs/plans: exactly these; a server that emits
// RequestReceived-only entries never yields a verdict from them).
const (
	StageRequestReceived  = "RequestReceived"
	StageResponseStarted  = "ResponseStarted"
	StageResponseComplete = "ResponseComplete"
	StagePanic            = "Panic"
)

// TerminalStage reports whether the stage concludes an API operation.
// ResponseStarted is tolerated for streaming verbs only (never canonical).
func TerminalStage(stage string) bool {
	return stage == StageResponseComplete || stage == StagePanic
}

// Event is one Kubernetes audit event (audit.k8s.io/v1). Unknown fields are
// preserved in Extra for forensic fidelity of non-redacted captures.
type Event struct {
	Level           string               `json:"level"`
	AuditID         string               `json:"auditID"`
	Stage           string               `json:"stage"`
	RequestURI      string               `json:"requestURI"`
	Verb            string               `json:"verb"`
	User            *User                `json:"user,omitempty"`
	UserExtra       json.RawMessage      `json:"userExtra,omitempty"`
	SourceIPs       []string             `json:"sourceIPs,omitempty"`
	UserAgent       string               `json:"userAgent,omitempty"`
	ObjectRef       *ObjectRef           `json:"objectRef,omitempty"`
	ResponseStatus  *ResponseStatus      `json:"responseStatus,omitempty"`
	StageTimestamps map[string]time.Time `json:"stageTimestamps,omitempty"`
	Timestamp       time.Time            `json:"requestReceivedTimestamp"`
	Annotations     map[string]string    `json:"annotations,omitempty"`
	RequestObject   json.RawMessage      `json:"requestObject,omitempty"`
	ResponseObject  json.RawMessage      `json:"responseObject,omitempty"`
}

// User identity fields.
type User struct {
	Username string   `json:"username"`
	UID      string   `json:"uid,omitempty"`
	Groups   []string `json:"groups,omitempty"`
}

// ObjectRef names the target of the operation.
type ObjectRef struct {
	Resource    string `json:"resource,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	Name        string `json:"name,omitempty"`
	APIGroup    string `json:"apiGroup,omitempty"`
	APIVersion  string `json:"apiVersion,omitempty"`
	Subresource string `json:"subresource,omitempty"`
}

// ResponseStatus mirrors k8s Status.
type ResponseStatus struct {
	Code   int    `json:"code,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// Key is the de-duplication identity: the spike proved (auditID, stage) is
// the only stable event key (identical auditIDs legitimately repeat across
// stages of one operation).
func (e Event) Key() string { return e.AuditID + "|" + e.Stage }

// EffectiveTimestamp prefers the stage-specific timestamp.
func (e Event) EffectiveTimestamp() time.Time {
	if ts, ok := e.StageTimestamps[e.Stage]; ok && !ts.IsZero() {
		return ts
	}
	return e.Timestamp
}

// Parse decodes newline-delimited audit JSON, skipping lines that fail to
// parse (counted by the caller via ParseStats).
type ParseStats struct {
	Lines  int
	Bad    int
	Events []Event
}

// Parse implements tolerant newline-delimited decoding.
func Parse(data []byte) ParseStats {
	var st ParseStats
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024) // audit Request-level lines can be big
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		st.Lines++
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil || ev.AuditID == "" {
			st.Bad++
			continue
		}
		st.Events = append(st.Events, ev)
	}
	return st
}

// Store accumulates events keyed by (auditID, stage), preserving first-seen
// order per auditID for stable windows.
type Store struct {
	byKey    map[string]Event
	opStages map[string]map[string]bool
}

// NewStore builds an empty store.
func NewStore() *Store {
	return &Store{byKey: map[string]Event{}, opStages: map[string]map[string]bool{}}
}

// Add ingests events; returns count of newly stored keys.
func (s *Store) Add(events []Event) int {
	added := 0
	for _, e := range events {
		k := e.Key()
		if _, dup := s.byKey[k]; dup {
			continue
		}
		s.byKey[k] = e
		if s.opStages[e.AuditID] == nil {
			s.opStages[e.AuditID] = map[string]bool{}
		}
		s.opStages[e.AuditID][e.Stage] = true
		added++
	}
	return added
}

// Len returns the stored event count.
func (s *Store) Len() int { return len(s.byKey) }

// HasTerminal reports whether the operation (auditID) reached a terminal
// stage (ResponseComplete or Panic). Ops whose terminal stage never showed
// up by the end of drain must mark coverage incomplete.
func (s *Store) HasTerminal(auditID string) bool {
	for stage := range s.opStages[auditID] {
		if TerminalStage(stage) {
			return true
		}
	}
	return false
}

// Ops returns all auditIDs sorted by first event timestamp.
func (s *Store) Ops() []string {
	seen := map[string]bool{}
	var ids []string
	for _, e := range s.All() {
		if !seen[e.AuditID] {
			seen[e.AuditID] = true
			ids = append(ids, e.AuditID)
		}
	}
	return ids
}

// All returns stored events sorted by effective timestamp then key.
func (s *Store) All() []Event {
	out := make([]Event, 0, len(s.byKey))
	for _, e := range s.byKey {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		ti, tj := out[i].EffectiveTimestamp(), out[j].EffectiveTimestamp()
		if !ti.Equal(tj) {
			return ti.Before(tj)
		}
		return out[i].Key() < out[j].Key()
	})
	return out
}

// Window selects operations whose canonical (terminal) event lies in
// [start, end] inclusive, anchored by marker events. A marker is any event
// (typically an authenticated 404 GET of a nonce-named ConfigMap emitted by
// the harness certificate identity). Returns the ops plus the ids of
// incomplete operations (no terminal stage by drain end).
type WindowResult struct {
	Ops        []Event
	Incomplete []string
	// DelegatedOps holds established CONNECT subresource observations
	// (pods/exec, attach, portforward, proxy). These streams legitimately
	// never flush ResponseComplete, but ADR 0001 lists them as sensitive:
	// a successful exec crosses a trust boundary into delegated workload
	// authority the audit stream cannot attribute downstream. They are
	// recorded authoritatively here and the engine refuses to qualify any
	// window containing them.
	DelegatedOps []Event
}

// Window filters the store by marker timestamps.
func (s *Store) Window(start, end Event) WindowResult {
	lo, hi := start.EffectiveTimestamp(), end.EffectiveTimestamp()
	byOp := map[string][]Event{}
	for _, e := range s.All() {
		byOp[e.AuditID] = append(byOp[e.AuditID], e)
	}
	var res WindowResult
	for _, opID := range s.Ops() {
		group := byOp[opID]
		var canonical Event
		found := false
		for _, e := range group {
			if TerminalStage(e.Stage) {
				canonical, found = e, true
				break
			}
		}
		if !found {
			// CONNECT subresources (exec/attach/portforward/proxy) never
			// flush ResponseComplete on a healthy long-lived stream —
			// ResponseStarted (the 101 upgrade) is their authoritative
			// establishment observation. Record it; never hide it.
			if cs := connectStarted(group); cs != nil {
				if t := cs.EffectiveTimestamp(); !t.Before(lo) && !t.After(hi) {
					res.Ops = append(res.Ops, *cs)
					res.DelegatedOps = append(res.DelegatedOps, *cs)
				}
				continue
			}
			if connectSubresource(group) {
				// connect attempt without even ResponseStarted: the
				// outcome is unknown — an incomplete observation.
				if len(group) > 0 {
					first := group[0]
					if !first.EffectiveTimestamp().Before(lo) && !first.EffectiveTimestamp().After(hi) {
						res.Incomplete = append(res.Incomplete, opID)
						res.Ops = append(res.Ops, first) // forensics
					}
				}
				continue
			}
			// Long-lived WATCH/LIST streams have no meaningful terminal
			// stage (client-side close races the ResponseComplete flush —
			// empirically the dominant audit "gap" on busy clusters, and
			// the pre-window variant was a proven false-positive source
			// with 121 controller watches). No mutation is ever expressed
			// through watch semantics. Tolerated wholesale.
			if streamingOp(group) {
				continue
			}
			// Only NON-streaming ops that BEGAN inside the window can make
			// that window incomplete: a plain get/list/mutation whose
			// terminal stage never arrived is a lost observation.
			if len(group) > 0 {
				first := group[0]
				if !first.EffectiveTimestamp().Before(lo) && !first.EffectiveTimestamp().After(hi) {
					res.Incomplete = append(res.Incomplete, opID)
					res.Ops = append(res.Ops, first) // forensics
				}
			}
			continue
		}
		ts := canonical.EffectiveTimestamp()
		if ts.Before(lo) || ts.After(hi) {
			continue
		}
		res.Ops = append(res.Ops, canonical)
		// A CONNECT channel that DID flush a terminal stage is still a
		// delegated execution: the audit stream sees the handshake and
		// the close, never what happened inside the pod. Reviewer round-2
		// blocker #1: the gate must key on the operation's nature, not
		// on whether the stream happened to terminate cleanly.
		if connectSubresource(group) {
			res.DelegatedOps = append(res.DelegatedOps, canonical)
		}
	}
	return res
}

// FindMarker locates a window-marker event: an audit event whose
// requestURI references the nonce marker object, attributed to username.
// Matching primarily on the nonce requestURI (spike finding: requestURI
// contains the exact ConfigMap name; correlation by Audit-ID header is the
// fallback the harness cannot fake from the client side).
func FindMarker(events []Event, username, nonce string) (Event, error) {
	for _, e := range events {
		if !strings.Contains(e.RequestURI, nonce) {
			continue
		}
		if username != "" && (e.User == nil || e.User.Username != username) {
			continue
		}
		if e.Stage != "" && !TerminalStage(e.Stage) {
			continue // anchor on terminal stages only
		}
		return e, nil
	}
	return Event{}, fmt.Errorf("audit: marker %q not observed (username %q)", nonce, username)
}

// streamingOp reports whether an op group is a watch or an attach-style
// stream, identified from any stage's verb / request URI.
func streamingOp(group []Event) bool {
	for _, e := range group {
		if strings.EqualFold(e.Verb, "watch") {
			return true
		}
		switch {
		case strings.Contains(e.RequestURI, "follow=true"):
			// log tailing is a read stream, not a delegated channel.
			return true
		}
	}
	return false
}

// connectSubresources are the pod-level channels that hand authority to a
// process inside a workload (or the apiserver itself, for proxy).
var connectSubresources = map[string]bool{
	"exec":        true,
	"attach":      true,
	"portforward": true,
	"proxy":       true,
}

// connectSubresource reports whether an op group targets a connect
// subresource, via ObjectRef.subresource first and URI as fallback.
func connectSubresource(group []Event) bool {
	for _, e := range group {
		if e.ObjectRef != nil && connectSubresources[e.ObjectRef.Subresource] {
			return true
		}
		for sub := range connectSubresources {
			if strings.Contains(e.RequestURI, "/"+sub) {
				return true
			}
		}
	}
	return false
}

// connectStarted returns the authoritative establishment observation for a
// connect group: the ResponseStarted stage (the 101 upgrade point).
// Returns nil when the group never got that far.
func connectStarted(group []Event) *Event {
	for i := range group {
		if group[i].Stage == StageResponseStarted && connectSubresource(group) {
			return &group[i]
		}
	}
	return nil
}
