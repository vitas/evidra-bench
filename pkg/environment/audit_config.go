package environment

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/vitas/evidra-bench/pkg/scenario"
)

// API-audit provisioning ported verbatim from the Linux-proven spike
// recipes in tests/spikes/audit-provisioning/ (FINDINGS.md). Two hard
// constraints drove the design:
//
//  1. DooD-safe staging: a runner container has no usable daemon-host
//     filesystem, so policy + kubeadm patch live in a NAMED docker volume
//     written by a throwaway `docker run alpine` (daemon-side writer) and
//     referenced from the cluster config via the volume's Mountpoint.
//     Nested host-file binds and in-container writes are proven dead ends.
//  2. Restart-free: audit flags must be present at the apiserver's FIRST
//     start. Patching a running apiserver's static pod cold-breaks bearer
//     authn for minutes (jwks refetch). For kind this means the v1beta3
//     kubeadm patch mechanism, which the node image must actually support:
//     kindest/node:v1.31.2 is the proven pin — newer kind runner defaults
//     (v1.37.0-era) silently drop v1beta3 patches and the audit args
//     disappear without any error. Silent evidence loss is exactly what
//     ADR 0001 forbids, so the pin is part of the contract, not a hint.

// DefaultAuditNodeImage is the proven kind node image for audit
// provisioning (see FINDINGS.md "node image pin").
const DefaultAuditNodeImage = "kindest/node:v1.31.2"

// Audit staging resources are named deterministically per cluster so
// Destroy can always clean them up without extra bookkeeping.
func auditVolumeName(clusterName string) string { return "evidra-audit-" + clusterName }

// AuditConfig asks a provider to provision API audit capture at cluster
// creation. Zero value means no audit (coverage will be reported absent —
// never silently skipped).
type AuditConfig struct {
	Enabled bool
	// PolicyMode selects the mutation-verb level: "" or "metadata" (the
	// default: request bodies never enter evidence) or "request" (request
	// bodies recorded; behind explicit opt-in, design §5).
	PolicyMode string
	// NodeImage pins the kind node image; empty uses DefaultAuditNodeImage.
	// Ignored by k3d (k3s bakes its own image).
	NodeImage string
}

// AuditPolicyYAML renders the audit policy. Non-mutation traffic is
// Metadata-level; configmap GETs (window markers) stay Metadata so the
// marker payload never leaks anything useful to a body-capture mode;
// mutations follow PolicyMode.
func AuditPolicyYAML(mode string) string {
	level := "Metadata"
	if mode == "request" {
		level = "Request"
	}
	return `apiVersion: audit.k8s.io/v1
kind: Policy
rules:
  - level: Metadata
    resources: [{group: "", resources: ["configmaps"]}]
  - level: ` + level + `
    verbs: ["create", "update", "patch", "delete", "deletecollection"]
  - level: Metadata
  - level: None
`
}

// kindAuditPodPatch is the kubeadm Pod-manifest patch adding the audit
// policy + log hostPaths to kube-apiserver at first start.
const kindAuditPodPatch = `apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver
spec:
  containers:
    - name: kube-apiserver
      volumeMounts:
        - mountPath: /etc/kubernetes/audit
          name: audit-policy
          readOnly: true
        - mountPath: /var/log/kubernetes
          name: audit-log
  volumes:
    - name: audit-policy
      hostPath:
        path: /etc/kubernetes/audit
    - name: audit-log
      hostPath:
        path: /var/log/kubernetes
        type: DirectoryOrCreate
`

// AuditVolume is the staged named docker volume backing audit config.
type AuditVolume struct {
	Name       string
	Mountpoint string // daemon-side path (stable on the docker node)
}

// StageAuditVolume creates and fills the per-cluster audit volume. All
// writes are daemon-side via throwaway containers (DooD-safe).
func StageAuditVolume(ctx context.Context, runner CommandRunner, clusterName, policyYAML string) (*AuditVolume, error) {
	name := auditVolumeName(clusterName)
	if out, err := runner.Run(ctx, exec.CommandContext(ctx, "docker", "volume", "create", name)); err != nil {
		return nil, fmt.Errorf("audit: volume create: %w: %s", err, out)
	}
	if err := stageFile(ctx, runner, name, "mkdir -p /data/patches && cat > /data/policy.yaml", policyYAML); err != nil {
		return nil, err
	}
	if err := stageFile(ctx, runner, name, "cat > /data/patches/kube-apiserver.yaml", kindAuditPodPatch); err != nil {
		return nil, err
	}
	out, err := runner.Run(ctx, exec.CommandContext(ctx, "docker", "volume", "inspect", "-f", "{{.Mountpoint}}", name))
	if err != nil {
		return nil, fmt.Errorf("audit: volume inspect: %w: %s", err, out)
	}
	mount := strings.TrimSpace(string(out))
	if mount == "" {
		return nil, fmt.Errorf("audit: volume %s has empty mountpoint", name)
	}
	return &AuditVolume{Name: name, Mountpoint: mount}, nil
}

func stageFile(ctx context.Context, runner CommandRunner, volume, script, content string) error {
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "-i", "-v", volume+":/data", "alpine:3.22", "sh", "-c", script)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("audit: stdin pipe: %w", err)
	}
	go func() {
		defer func() { _ = stdin.Close() }()
		_, _ = io.WriteString(stdin, content)
	}()
	out, err := runner.Run(ctx, cmd)
	if err != nil {
		return fmt.Errorf("audit: staging write: %w: %s", err, out)
	}
	return nil
}

// RemoveAuditVolume drops the staging volume (idempotent).
func RemoveAuditVolume(ctx context.Context, runner CommandRunner, clusterName string) error {
	out, err := runner.Run(ctx, exec.CommandContext(ctx, "docker", "volume", "rm", auditVolumeName(clusterName)))
	if err != nil && !strings.Contains(strings.ToLower(string(out)), "no such volume") {
		return fmt.Errorf("audit: volume rm: %w: %s", err, out)
	}
	return nil
}

// BuildKindAuditConfig renders a full kind Cluster config that mounts the
// staged volume into the control-plane node and wires the audit flags via
// v1beta3 kubeadm patches (InitConfiguration.patches.directory +
// ClusterConfiguration.apiServer.extraArgs). CNI settings from the legacy
// KubernetesConfig are preserved; worker nodes are only added when
// runtimes need them (the spike recipe is single-node).
func BuildKindAuditConfig(vol *AuditVolume, k8s scenario.KubernetesConfig) string {
	var b strings.Builder
	b.WriteString("kind: Cluster\napiVersion: kind.x-k8s.io/v1alpha4\n")
	if k8s.CNI != "" {
		b.WriteString("networking:\n  disableDefaultCNI: true\n")
		if k8s.CNI == "cilium" {
			b.WriteString("  kubeProxyMode: none\n")
		}
	}
	b.WriteString("nodes:\n  - role: control-plane\n")
	b.WriteString("    extraMounts:\n")
	b.WriteString("      - hostPath: " + vol.Mountpoint + "/policy.yaml\n")
	b.WriteString("        containerPath: /etc/kubernetes/audit/policy.yaml\n        readOnly: true\n")
	b.WriteString("      - hostPath: " + vol.Mountpoint + "/patches\n")
	b.WriteString("        containerPath: /etc/kubernetes/kubeadm-patches\n        readOnly: true\n")
	b.WriteString("    kubeadmConfigPatches:\n")
	b.WriteString("      - |\n")
	b.WriteString("        apiVersion: kubeadm.k8s.io/v1beta3\n        kind: InitConfiguration\n        patches:\n          directory: /etc/kubernetes/kubeadm-patches\n")
	b.WriteString("      - |\n")
	b.WriteString("        apiVersion: kubeadm.k8s.io/v1beta3\n        kind: ClusterConfiguration\n        metadata:\n          name: config\n        apiServer:\n          extraArgs:\n")
	b.WriteString("            audit-policy-file: /etc/kubernetes/audit/policy.yaml\n")
	b.WriteString("            audit-log-path: /var/log/kubernetes/audit.log\n")
	b.WriteString("            audit-log-maxage: \"1\"\n")
	b.WriteString("            audit-log-maxsize: \"16\"\n")
	b.WriteString("            audit-log-maxbackup: \"1\"\n")
	if len(k8s.Runtimes) > 0 {
		b.WriteString("  - role: worker\n")
		b.WriteString("    extraMounts:\n")
		for _, rt := range k8s.Runtimes {
			if rt.Name == "gvisor" {
				b.WriteString("      - hostPath: /usr/local/bin/runsc\n        containerPath: /usr/local/bin/runsc\n")
			}
		}
	}
	return b.String()
}

// K3dAuditArgs returns the k3d cluster-create flags provisioning the same
// audit setup (k3s embeds the apiserver: named volume + kube-apiserver-arg
// passthrough, no restart needed).
func K3dAuditArgs(volumeName string) []string {
	return []string{
		"--volume", volumeName + ":/etc/kubernetes/audit@server:0",
		"--k3s-arg", "--kube-apiserver-arg=audit-policy-file=/etc/kubernetes/audit/policy.yaml@server:0",
		"--k3s-arg", "--kube-apiserver-arg=audit-log-path=/var/log/kubernetes/audit.log@server:0",
		"--k3s-arg", "--kube-apiserver-arg=audit-log-maxage=1@server:0",
		"--k3s-arg", "--kube-apiserver-arg=audit-log-maxsize=16@server:0",
		"--k3s-arg", "--kube-apiserver-arg=audit-log-maxbackup=1@server:0",
	}
}

// kindInitPatch and kindClusterArgsPatch are the two v1beta3 kubeadm patch
// documents the proven recipe applies.
func kindInitPatch() string {
	return `apiVersion: kubeadm.k8s.io/v1beta3
kind: InitConfiguration
patches:
  directory: /etc/kubernetes/kubeadm-patches
`
}

func kindClusterArgsPatch() string {
	return `apiVersion: kubeadm.k8s.io/v1beta3
kind: ClusterConfiguration
metadata:
  name: config
apiServer:
  extraArgs:
    audit-policy-file: /etc/kubernetes/audit/policy.yaml
    audit-log-path: /var/log/kubernetes/audit.log
    audit-log-maxage: "1"
    audit-log-maxsize: "16"
    audit-log-maxbackup: "1"
`
}

// MergeKindAuditConfig augments a checked-in kind cluster config asset with
// the audit mounts + kubeadm patches, preserving every other user setting.
func MergeKindAuditConfig(assetYAML []byte, vol *AuditVolume) ([]byte, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(assetYAML, &doc); err != nil {
		return nil, fmt.Errorf("audit: parse kind asset: %w", err)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	if _, ok := doc["kind"]; !ok {
		doc["kind"] = "Cluster"
	}
	if _, ok := doc["apiVersion"]; !ok {
		doc["apiVersion"] = "kind.x-k8s.io/v1alpha4"
	}
	nodes, _ := doc["nodes"].([]any)
	var cp map[string]any
	for _, n := range nodes {
		if m, ok := n.(map[string]any); ok {
			if r, _ := m["role"].(string); r == "control-plane" || r == "" {
				cp = m
				break
			}
		}
	}
	if cp == nil {
		cp = map[string]any{"role": "control-plane"}
		nodes = append(nodes, cp)
		doc["nodes"] = nodes
	}
	mounts, _ := cp["extraMounts"].([]any)
	mounts = append(mounts,
		map[string]any{"hostPath": vol.Mountpoint + "/policy.yaml", "containerPath": "/etc/kubernetes/audit/policy.yaml", "readOnly": true},
		map[string]any{"hostPath": vol.Mountpoint + "/patches", "containerPath": "/etc/kubernetes/kubeadm-patches", "readOnly": true},
	)
	cp["extraMounts"] = mounts
	patches, _ := cp["kubeadmConfigPatches"].([]any)
	patches = append(patches, kindInitPatch(), kindClusterArgsPatch())
	cp["kubeadmConfigPatches"] = patches
	return yaml.Marshal(doc)
}
