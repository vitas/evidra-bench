package suite

import (
	"testing"
)

func TestDigestChangesWhenModelCapabilitiesChange(t *testing.T) {
	root := newTestProject(t)
	writeTestScenario(t, root, "kubernetes/repair", "repair", "repair prompt")

	baselinePath := writeTestManifest(t, root, validTestManifest("kubernetes/repair"))
	baseline, err := Load(baselinePath, root)
	if err != nil {
		t.Fatalf("Load() baseline error = %v", err)
	}

	capabilityPath := writeTestManifest(t, root, validTestManifest("kubernetes/repair")+`model:
  capabilities: [tools]
`)
	withCapabilities, err := Load(capabilityPath, root)
	if err != nil {
		t.Fatalf("Load() with capabilities error = %v", err)
	}

	if baseline.Digest == withCapabilities.Digest {
		t.Fatalf("digest must change when model capabilities change: %q", baseline.Digest)
	}
}

func TestDigestIsStableForIdenticalManifest(t *testing.T) {
	root := newTestProject(t)
	writeTestScenario(t, root, "kubernetes/repair", "repair", "repair prompt")
	manifestPath := writeTestManifest(t, root, validTestManifest("kubernetes/repair")+`model:
  capabilities: [tools]
`)

	first, err := Load(manifestPath, root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Load(manifestPath, root)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest != second.Digest {
		t.Fatalf("digest unstable: %q != %q", first.Digest, second.Digest)
	}
}
