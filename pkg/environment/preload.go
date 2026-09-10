package environment

import (
	"context"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var preloadImageRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._/:@-]*$`)

// PreloadFixtureImages pulls EVIDRA_PRELOAD_IMAGES (comma/space separated
// refs) into a cluster node's CRI store before any case starts. Cold
// fixture pulls through a throttled mirror race rollout waiters (found by
// the Phase 9 matrix: a 120s `rollout status` inside the agent lost to a
// second-replica pull). Opt-in only: unset env = zero behavior change.
// Failures are logged, never fatal — provisioning must not hard-depend on
// a warm cache.
func PreloadFixtureImages(ctx context.Context, nodeContainer string) {
	raw := strings.TrimSpace(os.Getenv("EVIDRA_PRELOAD_IMAGES"))
	if raw == "" || nodeContainer == "" {
		return
	}
	for _, img := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' }) {
		if !preloadImageRe.MatchString(img) {
			log.Printf("[preload] skipping malformed image ref %q", img)
			continue
		}
		cctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
		//nolint:gosec // validated ref; fixed argv.
		cmd := exec.CommandContext(cctx, "docker", "exec", nodeContainer, "crictl", "pull", img)
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("[preload] crictl pull %s on %s (non-fatal): %v: %s", img, nodeContainer, err, truncate(string(out), 200))
		} else {
			log.Printf("[preload] %s present on %s", img, nodeContainer)
		}
		cancel()
	}
}
