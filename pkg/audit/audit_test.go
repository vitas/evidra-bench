package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func ev(auditID, stage, uri, user, verb string, ts time.Time) Event {
	return Event{
		Level: "Metadata", AuditID: auditID, Stage: stage, RequestURI: uri,
		Verb: verb, User: &User{Username: user}, Timestamp: ts,
		StageTimestamps: map[string]time.Time{stage: ts},
		ObjectRef:       &ObjectRef{Resource: "configmaps", Namespace: "evidra-system", Name: lastSeg(uri)},
	}
}

func lastSeg(uri string) string {
	i := strings.LastIndex(uri, "/")
	if i < 0 {
		return uri
	}
	return uri[i+1:]
}

func jsonl(t *testing.T, events ...Event) []byte {
	t.Helper()
	var b strings.Builder
	for _, e := range events {
		data, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(data)
		b.WriteString("\n")
	}
	return []byte(b.String())
}

type memSource struct {
	name  string
	data  []byte
	reads int
	fail  error
}

func (m *memSource) NodeName() string { return m.name }
func (m *memSource) Read(context.Context) ([]byte, error) {
	m.reads++
	if m.fail != nil {
		return nil, m.fail
	}
	return m.data, nil
}

func TestStoreDedupSameAuditIDAcrossStages(t *testing.T) {
	t0 := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	// The spike proved one operation emits the SAME auditID for multiple
	// stages; the store key is (auditID, stage).
	st := NewStore()
	added := st.Add([]Event{
		ev("op1", StageRequestReceived, "/api/v1/namespaces/bench/pods/x", "agent", "get", t0),
		ev("op1", StageResponseStarted, "/api/v1/namespaces/bench/pods/x", "agent", "get", t0.Add(time.Millisecond)),
		ev("op1", StageResponseComplete, "/api/v1/namespaces/bench/pods/x", "agent", "get", t0.Add(2*time.Millisecond)),
	})
	if added != 3 || st.Len() != 3 {
		t.Fatalf("dedupe broken: added=%d len=%d", added, st.Len())
	}
	// Re-adding (poll re-reads the whole file) must be a no-op.
	if again := st.Add(st.All()); again != 0 {
		t.Fatalf("re-add not deduped: %d", again)
	}
	if !st.HasTerminal("op1") {
		t.Fatal("op1 lost its terminal stage")
	}
	// Panic-only op still terminal.
	st.Add([]Event{ev("op2", StagePanic, "/api/v1/x", "agent", "post", t0)})
	if !st.HasTerminal("op2") {
		t.Fatal("Panic must be terminal")
	}
	// RequestReceived-only op is NOT terminal.
	st.Add([]Event{ev("op3", StageRequestReceived, "/api/v1/y", "agent", "get", t0)})
	if st.HasTerminal("op3") {
		t.Fatal("RequestReceived alone must not be terminal")
	}
}

func TestWindowAndMarkers(t *testing.T) {
	t0 := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	start := ev("m1", StageResponseComplete, "/api/v1/namespaces/evidra-system/configmaps/evidra-marker-AAA", "system:admin", "get", t0)
	end := ev("m2", StageResponseComplete, "/api/v1/namespaces/evidra-system/configmaps/evidra-marker-BBB", "system:admin", "get", t0.Add(60*time.Second))
	inside := ev("a1", StageResponseComplete, "/apis/apps/v1/namespaces/bench/deployments/web", "system:serviceaccount:evidra-system:evidra-agent", "patch", t0.Add(30*time.Second))
	before := ev("z1", StageResponseComplete, "/apis/apps/v1/namespaces/bench/deployments/old", "u", "patch", t0.Add(-5*time.Second))
	after := ev("z2", StageResponseComplete, "/apis/apps/v1/namespaces/bench/deployments/next", "u", "patch", t0.Add(120*time.Second))
	// non-terminal op inside the window:
	hanging := ev("h1", StageRequestReceived, "/api/v1/namespaces/bench/secrets/s", "u", "get", t0.Add(31*time.Second))

	st := NewStore()
	st.Add([]Event{before, start, inside, after, hanging, end})

	sv, err := FindMarker(st.All(), "system:admin", "evidra-marker-AAA")
	if err != nil || sv.AuditID != "m1" {
		t.Fatalf("start marker: %v %+v", err, sv)
	}
	ev2, err := FindMarker(st.All(), "system:admin", "evidra-marker-BBB")
	_ = ev2
	if err != nil {
		t.Fatal(err)
	}
	// Wrong username must not match.
	if _, err := FindMarker(st.All(), "system:anonymous", "evidra-marker-AAA"); err == nil {
		t.Fatal("marker must be attributed")
	}

	res := st.Window(start, end)
	ids := map[string]bool{}
	for _, e := range res.Ops {
		ids[e.AuditID] = true
	}
	if !ids["a1"] || ids["z1"] || ids["z2"] {
		t.Fatalf("window membership wrong: %+v", ids)
	}
	if len(res.Incomplete) != 1 || res.Incomplete[0] != "h1" {
		t.Fatalf("incomplete ops = %+v, want [h1]", res.Incomplete)
	}
}

func TestCollectHappyPathComplete(t *testing.T) {
	t0 := time.Now().Add(-time.Minute)
	src := &memSource{name: "node1"}
	src.data = jsonl(t,
		ev("m1", StageResponseComplete, "/api/v1/namespaces/evidra-system/configmaps/evidra-marker-START1", "system:admin", "get", t0),
		ev("a1", StageResponseComplete, "/apis/apps/v1/namespaces/bench/deployments/web", "agent", "patch", t0.Add(time.Second)),
		ev("m2", StageResponseComplete, "/api/v1/namespaces/evidra-system/configmaps/evidra-marker-END2", "system:admin", "get", t0.Add(2*time.Second)),
	)
	res, err := Collect(context.Background(), CollectRequest{
		Sources:        []Source{src},
		StartNonce:     "evidra-marker-START1",
		EndNonce:       "evidra-marker-END2",
		MarkerUsername: "system:admin",
		Deadline:       time.Now().Add(5 * time.Second),
		Poll:           10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Coverage != CoverageComplete {
		t.Fatalf("coverage = %q (%v), want complete", res.Coverage, res.Reasons)
	}
	// Marker ops are inside their own window by construction; the slice
	// keeps them because they ARE the boundary evidence.
	ids := map[string]bool{}
	for _, e := range res.Window.Ops {
		ids[e.AuditID] = true
	}
	if !ids["a1"] || !ids["m1"] || !ids["m2"] || len(res.Window.Ops) != 3 {
		t.Fatalf("window = %+v", res.Window.Ops)
	}
}

func TestCollectCoverageDowngrades(t *testing.T) {
	t0 := time.Now().Add(-time.Minute)
	full := jsonl(t,
		ev("m1", StageResponseComplete, "/x/evidra-marker-S", "system:admin", "get", t0),
		ev("m2", StageResponseComplete, "/x/evidra-marker-E", "system:admin", "get", t0.Add(time.Second)),
	)
	t.Run("missing end marker", func(t *testing.T) {
		src := &memSource{name: "n", data: jsonl(t, ev("m1", StageResponseComplete, "/x/evidra-marker-S", "system:admin", "get", t0))}
		res, _ := Collect(context.Background(), CollectRequest{Sources: []Source{src}, StartNonce: "evidra-marker-S", EndNonce: "evidra-marker-E", MarkerUsername: "system:admin", Deadline: time.Now().Add(200 * time.Millisecond), Poll: 10 * time.Millisecond})
		if res.Coverage != CoverageIncomplete || strings.Join(res.Reasons, ",") != ReasonEndMarker {
			t.Fatalf("res = %+v", res)
		}
	})
	t.Run("non-terminal ops downgrade", func(t *testing.T) {
		src := &memSource{name: "n", data: jsonl(t,
			ev("m1", StageResponseComplete, "/x/evidra-marker-S", "system:admin", "get", t0),
			ev("h", StageRequestReceived, "/x/y", "a", "get", t0.Add(500*time.Millisecond)),
			ev("m2", StageResponseComplete, "/x/evidra-marker-E", "system:admin", "get", t0.Add(time.Second)),
		)}
		res, _ := Collect(context.Background(), CollectRequest{Sources: []Source{src}, StartNonce: "evidra-marker-S", EndNonce: "evidra-marker-E", MarkerUsername: "system:admin", Deadline: time.Now().Add(200 * time.Millisecond), Poll: 10 * time.Millisecond})
		if res.Coverage != CoverageIncomplete || !strings.Contains(strings.Join(res.Reasons, ","), ReasonNonTerminalOps) {
			t.Fatalf("res = %+v", res)
		}
	})
	t.Run("all readers failed => absent", func(t *testing.T) {
		src := &memSource{name: "n", fail: fmt.Errorf("docker exec: no such container")}
		res, _ := Collect(context.Background(), CollectRequest{Sources: []Source{src}, StartNonce: "S", EndNonce: "E", Deadline: time.Now().Add(200 * time.Millisecond), Poll: 10 * time.Millisecond})
		if res.Coverage != CoverageAbsent {
			t.Fatalf("res = %+v", res)
		}
	})
	t.Run("no sources => absent notProvisioned", func(t *testing.T) {
		res, _ := Collect(context.Background(), CollectRequest{})
		if res.Coverage != CoverageAbsent || res.Reasons[0] != ReasonNotProvisioned {
			t.Fatalf("res = %+v", res)
		}
	})
	_ = full
}

func TestRedactionCanary(t *testing.T) {
	canary := "SUPER-SECRET-TOKEN-9f3a"
	body := json.RawMessage(`{"metadata":{"name":"x"},"data":{"token":"` + canary + `"}}`)
	events := []Event{
		{Level: "Request", AuditID: "r1", Stage: StageResponseComplete, Verb: "create",
			RequestURI: "/api/v1/namespaces/bench/secrets", User: &User{Username: "agent"},
			RequestObject: body, ResponseObject: body,
			Annotations: map[string]string{"internal": canary}},
	}
	out, err := json.Marshal(Redacted(events))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), canary) {
		t.Fatalf("canary leaked through redaction: %s", out)
	}
	if !strings.Contains(string(out), `"auditID":"r1"`) {
		t.Fatalf("redaction destroyed metadata: %s", out)
	}
}

func TestParseTolerant(t *testing.T) {
	data := []byte(`{"auditID":"a","stage":"ResponseComplete","requestURI":"/x"}` + "\n" +
		`garbage line` + "\n" +
		`{"stage":"ResponseComplete","requestURI":"/no-audit-id"}` + "\n" +
		"\n")
	st := Parse(data)
	if st.Lines != 3 || st.Bad != 2 || len(st.Events) != 1 {
		t.Fatalf("stats = %+v", st)
	}
}

func TestMutationLevelPolicyProbe(t *testing.T) {
	lvl, err := MutationLevel(AuditPolicyForTest("Metadata"))
	if err != nil || lvl != "Metadata" {
		t.Fatalf("Metadata policy: %v %q", err, lvl)
	}
	lvl, err = MutationLevel(AuditPolicyForTest("Request"))
	if err != nil || lvl != "Request" {
		t.Fatalf("Request policy: %v %q", err, lvl)
	}
	if _, err := MutationLevel("kind: Nope"); err == nil {
		t.Fatal("must reject non-Policy yaml")
	}
}

// AuditPolicyForTest mirrors the provisioned policy shape.
func AuditPolicyForTest(level string) string {
	return `apiVersion: audit.k8s.io/v1
kind: Policy
rules:
  - level: Metadata
    resources: [{group: "", resources: ["configmaps"]}]
  - level: ` + level + `
    verbs: ["create", "update", "patch", "delete", "deletecollection"]
  - level: None
`
}

func TestWindowIgnoresPreExistingOpenStreams(t *testing.T) {
	t0 := time.Now().UTC()
	// A controller watch opened BEFORE the window start, never terminal.
	events := jsonl(t,
		ev("watch1", StageRequestReceived, "/apis/apps/v1/pods?watch=true", "system:kube-proxy", "watch", t0.Add(-time.Hour)),
		ev("m1", StageResponseComplete, "/x/evidra-marker-S", "system:admin", "get", t0),
		ev("op", StageResponseComplete, "/x/y", "a", "get", t0.Add(500*time.Millisecond)),
		ev("m2", StageResponseComplete, "/x/evidra-marker-E", "system:admin", "get", t0.Add(time.Second)),
	)
	store := NewStore()
	store.Add(Parse(events).Events)
	start, err := FindMarker(store.All(), "system:admin", "evidra-marker-S")
	if err != nil {
		t.Fatal(err)
	}
	end, err := FindMarker(store.All(), "system:admin", "evidra-marker-E")
	if err != nil {
		t.Fatal(err)
	}
	w := store.Window(start, end)
	if len(w.Incomplete) != 0 {
		t.Fatalf("pre-window stream penalized: %v", w.Incomplete)
	}
	if len(w.Ops) != 3 {
		t.Fatalf("ops = %d, want marker+op+marker", len(w.Ops))
	}
}
