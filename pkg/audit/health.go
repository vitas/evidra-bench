package audit

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Health is the post-provisioning verification result for one node.
//
// The spike proved a SILENT failure class: newer kubeadm versions drop
// v1beta3 pod patches without error, leaving a cluster that LOOKS fine but
// records no audit (kindest/node image pinning is the mitigation, this
// check is the backstop). A run whose audit plumbing is not verified can
// never be qualified, so every collector pass starts with health.
type Health struct {
	Node              string
	ManifestHasFlags  bool `json:"apiserver_args"`
	PolicyFilePresent bool
	LogDirWritable    bool
	Detail            string
}

// OK reports fully verified provisioning on the node.
func (h Health) OK() bool {
	return h.ManifestHasFlags && h.PolicyFilePresent && h.LogDirWritable
}

// VerifyNodeHealth probes one kind-style control-plane container via docker
// exec (DooD-safe, no mounts). probeCtx bounds each command.
func VerifyNodeHealth(ctx context.Context, container string) Health {
	h := Health{Node: container}
	run := func(script string) (string, error) {
		cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		//nolint:gosec // fixed probes, container from provisioning.
		cmd := exec.CommandContext(cctx, "docker", "exec", container, "sh", "-c", script)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	// Provider-neutral apiserver args probe: kind runs the apiserver as a
	// static pod (manifest), k3s embeds it in PID 1 (cmdline). The /proc
	// scan restricts to processes NAMED kube-apiserver or k3s — matching by
	// argv content would self-match this very probe script (its command
	// line legitimately contains the search string; the spike hit exactly
	// this trap).
	const argsProbe = `ok=; grep -q audit-policy-file /etc/kubernetes/manifests/kube-apiserver.yaml 2>/dev/null && ok=1; tr -d '\0' < /proc/1/cmdline 2>/dev/null | grep -q 'audit-policy-file' && ok=1; for c in /proc/[0-9]*/comm; do read n 2>/dev/null < "$c" || continue; if [ "$n" = kube-apiserver ] || [ "$n" = k3s ]; then tr -d '\0' < "${c%/comm}/cmdline" | grep -q 'audit-policy-file' && ok=1; fi; done; [ -n "$ok" ] && echo yes`
	if out, err := run(argsProbe); err != nil || !strings.Contains(out, "yes") {
		h.Detail = "apiserver lacks audit-policy-file arg (manifest or cmdline): " + trim(out)
		return h
	}
	h.ManifestHasFlags = true
	if out, err := run("test -f /etc/kubernetes/audit/policy.yaml && echo yes"); err != nil || !strings.Contains(out, "yes") {
		h.Detail = "audit policy file missing in node: " + trim(out)
		return h
	}
	h.PolicyFilePresent = true
	if out, err := run("mkdir -p /var/log/kubernetes && touch /var/log/kubernetes/.evidra-probe && rm -f /var/log/kubernetes/.evidra-probe && echo yes"); err != nil || !strings.Contains(out, "yes") {
		h.Detail = "audit log dir not writable: " + trim(out)
		return h
	}
	h.LogDirWritable = true
	return h
}

func trim(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}

// ErrUnhealthy summarizes failing nodes.
func ErrUnhealthy(healths []Health) error {
	var bad []string
	for _, h := range healths {
		if !h.OK() {
			bad = append(bad, fmt.Sprintf("%s: %s", h.Node, h.Detail))
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("audit health check failed: %s", strings.Join(bad, "; "))
	}
	return nil
}
