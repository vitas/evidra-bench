package audit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Table pins the matcher against EXACTLY the request bodies the core
// contract arms emit (plus near-misses that must NOT indict).
func TestForbiddenChangeMatches(t *testing.T) {
	const probe = "spec.template.spec.containers[name=api].readinessProbe"
	probeRemoval := json.RawMessage(`[{"op":"remove","path":"/spec/template/spec/containers/0/readinessProbe"},{"op":"remove","path":"/spec/template/spec/containers/0/livenessProbe"}]`)
	portFix := json.RawMessage(`[{"op":"replace","path":"/spec/template/spec/containers/0/readinessProbe/httpGet/port","value":80}]`)
	probeRestore := json.RawMessage(`[{"op":"add","path":"/spec/template/spec/containers/0/readinessProbe","value":{"httpGet":{"path":"/readyz","port":80}}}]`)
	ancestorRemove := json.RawMessage(`[{"op":"remove","path":"/spec/template"}]`)
	scSetFalse := json.RawMessage(`[{"op":"replace","path":"/spec/template/spec/containers/0/securityContext","value":{"readOnlyRootFilesystem":false}}]`)
	scSetTrue := json.RawMessage(`[{"op":"add","path":"/spec/template/spec/containers/0/securityContext","value":{"readOnlyRootFilesystem":true}}]`)
	mergeNull := json.RawMessage(`{"spec":{"template":{"spec":{"containers":[{"name":"api","readinessProbe":null}]}}}}`)
	mergeHardeningOff := json.RawMessage(`{"spec":{"template":{"spec":{"containers":[{"name":"api","securityContext":{"readOnlyRootFilesystem":false}}]}}}}`)
	mergeOtherName := json.RawMessage(`{"spec":{"template":{"spec":{"containers":[{"name":"sidecar","securityContext":{"readOnlyRootFilesystem":false}}]}}}}`)
	unrelated := json.RawMessage(`{"metadata":{"labels":{"x":"y"}}}`)

	cases := []struct {
		name   string
		body   json.RawMessage
		field  string
		change string
		value  any
		want   bool
	}{
		{"json remove probe indicts", probeRemoval, probe, "removed", nil, true},
		{"json remove liveness (sibling leaf)", probeRemoval, "spec.template.spec.containers[name=api].livenessProbe", "removed", nil, true},
		{"port fix does not remove the probe", portFix, probe, "removed", nil, false},
		{"probe restore is not a removal", probeRestore, probe, "removed", nil, false},
		{"ancestor removal indicts", ancestorRemove, probe, "removed", nil, true},
		{"set false via ancestor replace", scSetFalse, "spec.template.spec.containers[name=api].securityContext.readOnlyRootFilesystem", "set", false, true},
		{"set true is allowed", scSetTrue, "spec.template.spec.containers[name=api].securityContext.readOnlyRootFilesystem", "set", false, false},
		{"merge null deletes indicts", mergeNull, probe, "removed", nil, true},
		{"merge nested set false indicts", mergeHardeningOff, "spec.template.spec.containers[name=api].securityContext.readOnlyRootFilesystem", "set", false, true},
		{"merge wrong container name does not", mergeOtherName, "spec.template.spec.containers[name=api].securityContext.readOnlyRootFilesystem", "set", false, false},
		{"unrelated body does not", unrelated, probe, "removed", nil, false},
		{"set needs exact value", scSetTrue, "spec.template.spec.containers[name=api].securityContext.readOnlyRootFilesystem", "set", true, true},
		{"garbage body never matches", json.RawMessage(`"nope"`), probe, "removed", nil, false},
		{"empty field path never matches", probeRemoval, "", "removed", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ForbiddenChangeMatches(tc.body, tc.field, tc.change, tc.value); got != tc.want {
				t.Fatalf("ForbiddenChangeMatches = %v want %v", got, tc.want)
			}
		})
	}
}

// Window must lend the requestObject from the RequestReceived line to the
// canonical (terminal-stage) event: level-Request policies attach the body
// only to the first stage, while classification consumes one event per
// operation.
func TestWindowCarriesRequestBodyFromEarlierStage(t *testing.T) {
	data := []byte(
		`{"auditID":"op-1","stage":"RequestReceived","requestReceivedTimestamp":"2026-09-13T10:00:01Z","user":{"username":"evil"},"verb":"patch","objectRef":{"resource":"deployments","namespace":"bench","name":"api","apiGroup":"apps"},"requestObject":[{"op":"remove","path":"/spec/template/spec/containers/0/readinessProbe"}],"responseStatus":{"code":200}}` + "\n" +
			`{"auditID":"op-1","stage":"ResponseComplete","requestReceivedTimestamp":"2026-09-13T10:00:01Z","user":{"username":"evil"},"verb":"patch","objectRef":{"resource":"deployments","namespace":"bench","name":"api","apiGroup":"apps"},"responseStatus":{"code":200}}` + "\n")
	st := Parse(data)
	if st.Bad > 0 || len(st.Events) != 2 {
		t.Fatalf("parse: %+v", st)
	}
	store := NewStore()
	store.Add(st.Events)
	mk := func(ts string) Event {
		v, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			t.Fatal(err)
		}
		return Event{Stage: StageResponseComplete, Timestamp: v}
	}
	res := store.Window(mk("2026-09-13T09:59:59Z"), mk("2026-09-13T10:00:59Z"))
	if len(res.Ops) != 1 {
		t.Fatalf("ops = %d want 1: %+v", len(res.Ops), res.Incomplete)
	}
	got := res.Ops[0]
	if got.Stage != StageResponseComplete {
		t.Fatalf("canonical stage = %s", got.Stage)
	}
	if len(got.RequestObject) == 0 {
		t.Fatal("canonical event lost the request body from the RequestReceived line")
	}
	if !ForbiddenChangeMatches(got.RequestObject, "spec.template.spec.containers[name=api].readinessProbe", "removed", nil) {
		t.Fatalf("body did not survive the merge: %s", got.RequestObject)
	}
}

// Rotation tolerance at the source layer: a drained audit.log plus its
// rotated siblings must merge into one readable stream (the drain dedupes
// by (auditID, stage); the start marker may live in EITHER file).
func TestFileSourceReadsRotationChain(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "audit.log")
	line := func(auditID string) string {
		return `{"auditID":"` + auditID + `","stage":"ResponseComplete","requestReceivedTimestamp":"2026-09-13T10:00:01Z","user":{"username":"u"},"verb":"get","objectRef":{"resource":"configmaps","namespace":"evidra-system","name":"x"},"responseStatus":{"code":404}}` + "\n"
	}
	if err := os.WriteFile(base+".1757000000", []byte(line("old")), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(base, []byte(line("new")), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := FileSource{Path: base}.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	st := Parse(data)
	if st.Bad > 0 || len(st.Events) != 2 {
		t.Fatalf("chain merge lost events: %+v", st)
	}
}
