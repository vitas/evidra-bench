package verifier

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseProtocolOutputValid(t *testing.T) {
	cases := []struct {
		name   string
		out    string
		status string
	}{
		{"pass", `{"status":"pass","assertions":[{"name":"image-fixed","passed":true,"observed":"nginx:1.27"}]}`, "pass"},
		{"fail mixed", `{"status":"fail","assertions":[{"name":"a","passed":true},{"name":"b","passed":false,"observed":"x"}]}`, "fail"},
		{"error rbac", `{"status":"error","error":{"kind":"rbac","message":"forbidden"}}`, "error"},
		{"noise before doc", "kubectl warned about something\n" + `{"status":"pass","assertions":[{"name":"a","passed":true}]}` + "\n", "pass"},
	}
	for _, c := range cases {
		doc, ok := ParseProtocolOutput([]byte(c.out))
		if !ok || doc.Status != c.status {
			t.Fatalf("%s: ok=%v doc=%+v", c.name, ok, doc)
		}
	}
}

func TestParseProtocolOutputRejectsInvalid(t *testing.T) {
	invalid := []string{
		``,
		`PASS`,
		`{"status":"maybe"}`,
		`{"status":"pass"}`, // zero assertions
		`{"status":"pass","assertions":[{"name":"","passed":true}]}`,   // nameless
		`{"status":"pass","assertions":[{"name":"a","passed":false}]}`, // contradictory
		`{"status":"fail","assertions":[{"name":"a","passed":true}]}`,  // fail without a failed assertion
		`{"status":"error"}`,                          // no error member
		`{"status":"error","error":{"kind":"vibes"}}`, // unknown kind
		`{"status":"pass","error":{"kind":"rbac"},"assertions":[{"name":"a","passed":true}]}`,
	}
	for _, in := range invalid {
		if doc, ok := ParseProtocolOutput([]byte(in)); ok {
			t.Fatalf("must reject %q, got %+v", in, doc)
		}
	}
}

func writeScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "assert.sh")
	if err := os.WriteFile(path, []byte("#!/usr/bin/env bash\n"+body), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAssertV2CheckClassification(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		wantVerdict Verdict
		wantKind    string
	}{
		{"pass", `echo '{"status":"pass","assertions":[{"name":"a","passed":true}]}'`, VerdictPass, ""},
		// status is authoritative even when the script's exit code says
		// otherwise (wrapper honesty).
		{"pass despite nonzero exit", `echo '{"status":"pass","assertions":[{"name":"a","passed":true}]}'; exit 7`, VerdictPass, ""},
		{"fail despite zero exit", `echo '{"status":"fail","assertions":[{"name":"a","passed":false}]}'; exit 0`, VerdictFail, ""},
		{"unparseable => parse error", `echo "looks fine to me"`, VerdictError, ErrorKindParse},
		{"script error kind passes through", `echo '{"status":"error","error":{"kind":"timeout","message":"kubectl stalled"}}'`, VerdictError, ErrorKindTimeout},
	}
	for _, c := range cases {
		script := writeScript(t, c.body)
		res := (&AssertV2Check{Name: c.name, Command: script}).Check(context.Background(), "/nonexistent/kubeconfig")
		if res.Verdict != c.wantVerdict {
			t.Fatalf("%s: verdict = %q (%+v), want %q", c.name, res.Verdict, res, c.wantVerdict)
		}
		if c.wantKind == "" {
			if res.Error != nil {
				t.Fatalf("%s: unexpected error member %+v", c.name, res.Error)
			}
			continue
		}
		if res.Error == nil || res.Error.Kind != c.wantKind {
			t.Fatalf("%s: want error kind %q, got %+v", c.name, c.wantKind, res.Error)
		}
	}
}

func TestAssertV2MissingScriptIsTransportError(t *testing.T) {
	res := (&AssertV2Check{Name: "gone", Command: filepath.Join(t.TempDir(), "nope.sh")}).Check(context.Background(), "")
	if res.Verdict != VerdictError || res.Error.Kind != ErrorKindTransport {
		t.Fatalf("res = %+v", res)
	}
}

func TestAssertV2Timeout(t *testing.T) {
	script := writeScript(t, `sleep 5; echo '{"status":"pass","assertions":[{"name":"a","passed":true}]}'`)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	res := (&AssertV2Check{Name: "slow", Command: script}).Check(ctx, "")
	if res.Verdict != VerdictError {
		t.Fatalf("verdict = %q, want error (message %q)", res.Verdict, res.Message)
	}
	if res.Error.Kind != ErrorKindTimeout {
		t.Fatalf("kind = %q, want timeout (res %+v)", res.Error.Kind, res)
	}
}

func TestCommandSucceedsLegacyExitPolicy(t *testing.T) {
	cases := []struct {
		code int
		want Verdict
	}{
		{0, VerdictPass},
		{1, VerdictFail},
		{2, VerdictError},
		{127, VerdictError},
	}
	for _, c := range cases {
		res := (&CommandSucceedsCheck{Name: fmt.Sprint(c.code), Command: fmt.Sprintf("bash -c 'exit %d'", c.code)}).Check(context.Background(), "")
		if res.Verdict != c.want {
			t.Fatalf("exit %d: verdict = %q, want %q", c.code, res.Verdict, c.want)
		}
	}
}

func TestRunChecksErrorFlipsPassedAndIsQueryable(t *testing.T) {
	script := writeScript(t, `echo garbage`)
	vr := RunChecks(context.Background(), "", []Checker{
		&AssertV2Check{Name: "x", Command: script},
	})
	if vr.Passed {
		t.Fatal("errored check must flip Passed=false")
	}
	if len(vr.Errored()) != 1 {
		t.Fatalf("Errored() = %+v", vr.Errored())
	}
}

func TestBuildCheckersAcceptsAssertV2(t *testing.T) {
	checkers, err := BuildCheckers([]CheckDef{{Type: "assert-v2", Name: "a", Condition: "true"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(checkers) != 1 {
		t.Fatalf("checkers = %d", len(checkers))
	}
	if _, ok := checkers[0].(*AssertV2Check); !ok {
		t.Fatalf("type = %T", checkers[0])
	}
}
