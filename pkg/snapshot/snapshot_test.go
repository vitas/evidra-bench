package snapshot

import (
	"encoding/json"
	"strings"
	"testing"
)

func pod(name, rv string, ts string) map[string]any {
	return map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name": name, "namespace": "bench", "uid": "u-" + rv, "resourceVersion": rv,
			"creationTimestamp": ts,
			"annotations": map[string]any{
				"control-plane.alpha.kubernetes.io/leader": `{"holder":"x"}`,
				"team": "sre",
			},
			"managedFields": []any{map[string]any{"manager": "kubelet"}},
		},
		"spec": map[string]any{"containers": []any{map[string]any{"name": "c", "image": "nginx:1.27"}}},
		"status": map[string]any{
			"phase": "Running",
			"conditions": []any{map[string]any{
				"type": "Ready", "status": "True",
				"lastTransitionTime": ts,
			}},
		},
	}
}

func TestNormalizedDigestIsByteStable(t *testing.T) {
	ka, da := NormalizeObject(pod("web-1", "100", "2026-09-10T08:00:00Z"), "Pod", "bench")
	_, db := NormalizeObject(pod("web-1", "444", "2026-09-10T09:30:00Z"), "Pod", "bench")
	if ka.Kind != "Pod" {
		t.Fatalf("kind fallback broken: %+v", ka)
	}
	if string(da) != string(db) {
		t.Fatalf("volatile fields leaked:\n%s\n%s", da, db)
	}
	if strings.Contains(string(da), "resourceVersion") || strings.Contains(string(da), "managedFields") {
		t.Fatalf("volatile keys survived: %s", da)
	}
	// The semantic payload stays.
	if !strings.Contains(string(da), "nginx:1.27") || !strings.Contains(string(da), `"team":"sre"`) {
		t.Fatalf("content lost: %s", da)
	}
}

func TestRealChangeChangesDigest(t *testing.T) {
	o1 := pod("web-1", "1", "2026-09-10T08:00:00Z")
	o2 := pod("web-1", "2", "2026-09-10T08:00:00Z")
	o2["spec"].(map[string]any)["containers"] = []any{map[string]any{"name": "c", "image": "nginx:1.28"}}
	_, d1 := NormalizeObject(o1, "Pod", "bench")
	_, d2 := NormalizeObject(o2, "Pod", "bench")
	if string(d1) == string(d2) {
		t.Fatal("image change not detected")
	}
}

func TestSecretRedactionKeepsChangeDetection(t *testing.T) {
	mk := func(val string) map[string]any {
		return map[string]any{
			"kind": "Secret",
			"metadata": map[string]any{"name": "api-keys", "namespace": "bench",
				"resourceVersion": "7", "uid": "x"},
			"data": map[string]any{"key": val},
		}
	}
	k1, d1 := NormalizeObject(mk("YQ=="), "Secret", "bench")
	if k1.Kind != "Secret" {
		t.Fatalf("key = %+v", k1)
	}
	if strings.Contains(string(d1), "YQ==") {
		t.Fatalf("secret payload persisted: %s", d1)
	}
	if !strings.Contains(string(d1), "evidra:digest") {
		t.Fatalf("digest marker missing: %s", d1)
	}
	_, d2 := NormalizeObject(mk("Yg=="), "Secret", "bench")
	if string(d1) == string(d2) {
		t.Fatal("secret rotation invisible")
	}
}

func setOf(name string, objs ...map[string]any) *Set {
	s := NewSet(name, []Target{{Namespace: "bench", Resource: "pods", Kind: "Pod"}}, func(Target) (map[string]any, error) {
		return map[string]any{"items": toAny(objs)}, nil
	})
	return s
}

func toAny(objs []map[string]any) []any {
	out := make([]any, len(objs))
	for i, o := range objs {
		out[i] = o
	}
	return out
}

func TestDiffScopeClassification(t *testing.T) {
	before := setOf("baseline", pod("web-1", "1", "2026-09-10T08:00:00Z"), pod("db-1", "1", "2026-09-10T08:00:00Z"))
	afterObjs := []map[string]any{
		func() map[string]any {
			p := pod("web-1", "2", "2026-09-10T08:00:00Z")
			p["spec"] = map[string]any{"x": 1}
			return p
		}(),
		func() map[string]any {
			p := pod("db-1", "2", "2026-09-10T08:00:00Z")
			p["spec"] = map[string]any{"evil": true}
			return p
		}(),
		pod("new-pod", "5", "2026-09-10T08:00:00Z"),
	}
	after := setOf("post-agent", afterObjs...)
	allowed := func(k Key) bool { return k.Name == "web-1" || k.Name == "new-pod" }
	vs, ok := Diff(before, after, allowed)
	if ok != 2 {
		t.Fatalf("allowedChanges = %d, want 2", ok)
	}
	if len(vs) != 1 || vs[0].Object != "Pod/bench/db-1" || vs[0].Kind != "modified" {
		t.Fatalf("violations = %+v", vs)
	}
	if vs[0].Source != "snapshot" || vs[0].From == "" || vs[0].To == "" {
		t.Fatalf("fields = %+v", vs[0])
	}
}

func TestDiffDeleteAndCreate(t *testing.T) {
	before := setOf("b", pod("gone", "1", "2026-09-10T08:00:00Z"))
	after := setOf("a", pod("fresh", "1", "2026-09-10T08:00:00Z"))
	vs, _ := Diff(before, after, func(Key) bool { return false })
	if len(vs) != 2 {
		t.Fatalf("vs = %+v", vs)
	}
	if vs[0].Kind != "created" || vs[0].Object != "Pod/bench/fresh" { // sorted: fresh < gone
		t.Fatalf("first = %+v", vs[0])
	}
	if vs[1].Kind != "deleted" || vs[1].Object != "Pod/bench/gone" {
		t.Fatalf("second = %+v", vs[1])
	}
}

func TestSetDigestStableAcrossMapOrder(t *testing.T) {
	a := setOf("x", pod("p1", "1", "2026-09-10T08:00:00Z"), pod("p2", "1", "2026-09-10T08:00:00Z"))
	raw := `{"kind":"Pod","apiVersion":"v1","metadata":{"name":"p1","namespace":"bench","resourceVersion":"1"},"spec":{"a":1,"b":2}}`
	var m1 map[string]any
	if err := json.Unmarshal([]byte(raw), &m1); err != nil {
		t.Fatal(err)
	}
	_, dm := NormalizeObject(m1, "Pod", "bench")
	if !strings.Contains(string(dm), `"a":1,"b":2`) {
		t.Fatalf("key order not canonical: %s", dm)
	}
	if a.Digest() == "" || len(a.Digest()) != 64 {
		t.Fatalf("digest = %q", a.Digest())
	}
	b := setOf("x", pod("p2", "1", "2026-09-10T08:00:00Z"), pod("p1", "1", "2026-09-10T08:00:00Z"))
	if a.Digest() != b.Digest() {
		t.Fatal("object order leaked into digest")
	}
}

func TestNewSetTracksUnreadable(t *testing.T) {
	s := NewSet("x", []Target{{Namespace: "bench", Resource: "pods", Kind: "Pod"}, {Namespace: "bench", Resource: "secrets", Kind: "Secret"}}, func(tg Target) (map[string]any, error) {
		if tg.Resource == "secrets" {
			return nil, errForbidden
		}
		return map[string]any{"items": toAny([]map[string]any{pod("p", "1", "2026-09-10T08:00:00Z")})}, nil
	})
	if len(s.Unreadable) != 1 || !strings.Contains(s.Unreadable[0], "secrets") {
		t.Fatalf("unreadable = %v", s.Unreadable)
	}
	if s.Empty() || len(s.Objects) != 1 {
		t.Fatalf("objects = %v", s.Objects)
	}
}

var errForbidden = errorString("forbidden")

type errorString string

func (e errorString) Error() string { return string(e) }
