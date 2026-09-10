package environment

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMergeKindAuditConfigPreservesAsset(t *testing.T) {
	raw, err := os.ReadFile("../../clusters/kind/default.yaml")
	if err != nil {
		t.Fatal(err)
	}
	out, err := MergeKindAuditConfig(raw, &AuditVolume{Name: "v", Mountpoint: "/mp"})
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Kind  string `yaml:"kind"`
		Nodes []struct {
			Role                 string           `yaml:"role"`
			ExtraMounts          []map[string]any `yaml:"extraMounts"`
			KubeadmConfigPatches []string         `yaml:"kubeadmConfigPatches"`
		} `yaml:"nodes"`
	}
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Kind != "Cluster" || len(doc.Nodes) != 1 || doc.Nodes[0].Role != "control-plane" {
		t.Fatalf("asset shape lost: %s", out)
	}
	if len(doc.Nodes[0].ExtraMounts) != 2 || len(doc.Nodes[0].KubeadmConfigPatches) != 2 {
		t.Fatalf("audit bits missing: %s", out)
	}
	if !strings.Contains(string(out), "/mp/policy.yaml") {
		t.Fatalf("mountpoint not threaded: %s", out)
	}
}

func TestAuditPolicyMutationLevels(t *testing.T) {
	if !strings.Contains(AuditPolicyYAML(""), "- level: Metadata\n    verbs") {
		t.Fatalf("default policy must be Metadata-only for mutations:\n%s", AuditPolicyYAML(""))
	}
	if !strings.Contains(AuditPolicyYAML("request"), "- level: Request\n    verbs") {
		t.Fatalf("request mode must capture request bodies:\n%s", AuditPolicyYAML("request"))
	}
}
