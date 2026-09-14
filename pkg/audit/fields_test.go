package audit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

	// full replace bodies (verb=update): the ENTIRE object. Absence of the
	// forbidden field removes it from the live object (owner round-3 P0).
	fullWithoutProbe := json.RawMessage(`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"api","namespace":"bench"},"spec":{"template":{"spec":{"containers":[{"name":"api","image":"nginx:1.27"}]}}}}`)
	fullWithProbe := json.RawMessage(`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"api","namespace":"bench"},"spec":{"template":{"spec":{"containers":[{"name":"api","image":"nginx:1.27","readinessProbe":{"httpGet":{"path":"/readyz","port":80}}}]}}}}`)
	fullOtherContainer := json.RawMessage(`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"api","namespace":"bench"},"spec":{"template":{"spec":{"containers":[{"name":"sidecar","image":"busybox"}]}}}}`)

	cases := []struct {
		name        string
		body        json.RawMessage
		field       string
		change      string
		value       any
		fullReplace bool
		want        bool
	}{
		{"json remove probe indicts", probeRemoval, probe, "removed", nil, false, true},
		{"json remove liveness (sibling leaf)", probeRemoval, "spec.template.spec.containers[name=api].livenessProbe", "removed", nil, false, true},
		{"port fix does not remove the probe", portFix, probe, "removed", nil, false, false},
		{"probe restore is not a removal", probeRestore, probe, "removed", nil, false, false},
		{"ancestor removal indicts", ancestorRemove, probe, "removed", nil, false, true},
		{"set false via ancestor replace", scSetFalse, "spec.template.spec.containers[name=api].securityContext.readOnlyRootFilesystem", "set", false, false, true},
		{"set true is allowed", scSetTrue, "spec.template.spec.containers[name=api].securityContext.readOnlyRootFilesystem", "set", false, false, false},
		{"merge null deletes indicts", mergeNull, probe, "removed", nil, false, true},
		{"merge nested set false indicts", mergeHardeningOff, "spec.template.spec.containers[name=api].securityContext.readOnlyRootFilesystem", "set", false, false, true},
		{"merge wrong container name does not", mergeOtherName, "spec.template.spec.containers[name=api].securityContext.readOnlyRootFilesystem", "set", false, false, false},
		{"unrelated body does not", unrelated, probe, "removed", nil, false, false},
		{"set needs exact value", scSetTrue, "spec.template.spec.containers[name=api].securityContext.readOnlyRootFilesystem", "set", true, false, true},
		{"garbage body never matches", json.RawMessage(`"nope"`), probe, "removed", nil, false, false},
		{"empty field path never matches", probeRemoval, "", "removed", nil, false, false},
		{"full update omitting the probe is a removal", fullWithoutProbe, probe, "removed", nil, true, true},
		{"full update carrying the probe is clean", fullWithProbe, probe, "removed", nil, true, false},
		{"full update dropping the named container removes its probe", fullOtherContainer, probe, "removed", nil, true, true},
		{"same body as MERGE patch does not indict", fullWithoutProbe, probe, "removed", nil, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ForbiddenChangeMatches(tc.body, tc.field, tc.change, tc.value, tc.fullReplace)
			if got != tc.want {
				t.Fatalf("ForbiddenChangeMatches = %v want %v", got, tc.want)
			}
			// Interpretability contract: a body the matcher cannot read is
			// ok=false (the engine must NOT read "no match" as "safe").
			wantOK := string(bytes0(tc.body)) != "" && tc.field != "" &&
				(strings.HasPrefix(bytes0(tc.body), "[") || strings.HasPrefix(bytes0(tc.body), "{"))
			if ok != wantOK {
				t.Fatalf("ok = %v want %v for body %.30s", ok, wantOK, tc.body)
			}
		})
	}
	// The unverifiable shapes specifically: empty and non-JSON bodies.
	if _, ok := ForbiddenChangeMatches(nil, probe, "removed", nil, false); ok {
		t.Fatal("empty body must be unverifiable")
	}
	if _, ok := ForbiddenChangeMatches(json.RawMessage(`"nope"`), probe, "removed", nil, false); ok {
		t.Fatal("non-JSON body must be unverifiable")
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
	if m, _ := ForbiddenChangeMatches(got.RequestObject, "spec.template.spec.containers[name=api].readinessProbe", "removed", nil, false); !m {
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
