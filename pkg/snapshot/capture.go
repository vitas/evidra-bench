package snapshot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// KubectlLister reads targets through the evidence-reader kubeconfig with
// plain `kubectl get -o json` (the reader's grants define what exists in a
// snapshot; forbidden targets surface as Unreadable, never as silence).
type KubectlLister struct {
	KubeconfigPath string
	Timeout        time.Duration // per read
}

// List implements the ListFunc contract for NewSet.
func (k KubectlLister) List(ctx context.Context) ListFunc {
	timeout := k.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return func(t Target) (map[string]any, error) {
		cctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		args := []string{"--kubeconfig", k.KubeconfigPath, "get", t.Resource, "-n", t.Namespace, "-o", "json"}
		if t.Namespace == "" {
			args = []string{"--kubeconfig", k.KubeconfigPath, "get", t.Resource, "-A", "-o", "json"}
		}
		//nolint:gosec // fixed argv shape; inputs are profile-declared kinds.
		cmd := exec.CommandContext(cctx, "kubectl", args...)
		var out, errb bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errb
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("kubectl get %s -n %s: %v: %s", t.Resource, t.Namespace, err, truncate(errb.String(), 200))
		}
		var doc map[string]any
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			return nil, fmt.Errorf("kubectl get %s: parse: %w", t.Resource, err)
		}
		return doc, nil
	}
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}
