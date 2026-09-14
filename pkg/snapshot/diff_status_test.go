package snapshot

import (
	"encoding/json"
	"testing"
)

func obj(ns, name, image, ready string) []byte {
	o := map[string]any{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]any{"name": name, "namespace": ns},
		"spec":     map[string]any{"template": map[string]any{"spec": map[string]any{"containers": []any{map[string]any{"image": image}}}}},
	}
	if ready != "" {
		o["status"] = map[string]any{"readyReplicas": ready}
	}
	b, _ := json.Marshal(o)
	return b
}

// A protected workload recovering from a crash-loop rewrites its status
// while its spec never moves: that is controller output, not a persistent
// agent effect. The comparison must ignore status.
func TestDiffIgnoresStatusOnlyChanges(t *testing.T) {
	k := Key{Kind: "Deployment", Namespace: "bench", Name: "api"}
	before := &Set{Objects: map[Key][]byte{k: obj("bench", "api", "nginx:1.27-alpine", "0")}}
	after := &Set{Objects: map[Key][]byte{k: obj("bench", "api", "nginx:1.27-alpine", "2")}}
	v, allowed := Diff(before, after, nil)
	if len(v) != 0 || allowed != 0 {
		t.Fatalf("status-only churn reported violations: %v", v)
	}
	// ...while a real spec change still reports one.
	moved := &Set{Objects: map[Key][]byte{k: obj("bench", "api", "nginx:99", "2")}}
	v, _ = Diff(before, moved, nil)
	if len(v) != 1 || v[0].Kind != "modified" {
		t.Fatalf("spec change not detected: %v", v)
	}
}
