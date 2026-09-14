package environment

import (
	"strings"
	"testing"
)

func TestRewriteKubeconfigServer(t *testing.T) {
	in := []byte(`apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: AAAA
    server: https://127.0.0.1:54686
  name: kind-evidra
contexts:
- context:
    cluster: kind-evidra
    user: evidra-agent
  name: e
current-context: e
kind: Config
users:
- name: evidra-agent
  user:
    token: abc
`)
	out := string(rewriteKubeconfigServer(in, "https://evidra-x-control-plane:6443"))
	if !strings.Contains(out, "server: https://evidra-x-control-plane:6443") {
		t.Fatalf("server not rewritten:\n%s", out)
	}
	if strings.Contains(out, "127.0.0.1") {
		t.Fatalf("loopback server survived:\n%s", out)
	}
	if !strings.Contains(out, "\n    server: https://evidra-x") ||
		!strings.Contains(out, "token: abc") ||
		!strings.Contains(out, "certificate-authority-data: AAAA") {
		t.Fatalf("unexpected structural change:\n%s", out)
	}
}

func TestRewriteKubeconfigServerJSON(t *testing.T) {
	in := []byte(`{"apiVersion":"v1","clusters":[{"name":"kind-x","cluster":{"server":"https://127.0.0.1:60807","certificate-authority-data":"AAAA"}}],"contexts":[{"name":"c","context":{"cluster":"kind-x","user":"u"}}],"current-context":"c","users":[{"name":"u","user":{"token":"abc"}}]}`)
	out := string(rewriteKubeconfigServer(in, "https://evidra-x-control-plane:6443"))
	if strings.Contains(out, "127.0.0.1") {
		t.Fatalf("json loopback server survived: %s", out)
	}
	if !strings.Contains(out, `"server":"https://evidra-x-control-plane:6443"`) {
		t.Fatalf("json server not rewritten: %s", out)
	}
	if !strings.Contains(out, "token") || !strings.Contains(out, "AAAA") {
		t.Fatalf("json document damaged: %s", out)
	}
}
