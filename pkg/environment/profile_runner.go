package environment

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"samebits.com/evidra-infra-bench/pkg/scenario"
)

// ProfileRunner executes profile lifecycle hooks (install, healthcheck, cleanup)
// and parses lease.env output produced by install scripts.
type ProfileRunner struct{}

// ProfileRunRequest describes the inputs for preparing a profile.
type ProfileRunRequest struct {
	Assets      ProfileAssets
	Profile     scenario.ExecutionProfile
	Provider    string
	ClusterName string
	Kubeconfig  string
	ExtraEnv    map[string]string // additional env vars (e.g., EVIDRA_LOCALSTACK_SERVICES)
}

// ProfileRunResult holds the output of a successful profile preparation.
type ProfileRunResult struct {
	// ExtraEnv contains KEY=VALUE strings parsed from lease.env.
	ExtraEnv []string

	// Release runs cleanup.sh (best-effort) and removes the work directory.
	// It is safe to call multiple times; only the first call takes effect.
	Release func(context.Context) error
}

// Prepare creates a temporary work directory, runs install.sh then healthcheck.sh,
// parses lease.env from the work directory, and returns a result whose Release
// function will run cleanup.sh and remove the work directory.
//
// On install or healthcheck failure, best-effort cleanup runs before returning the error.
func (r *ProfileRunner) Prepare(ctx context.Context, req ProfileRunRequest) (*ProfileRunResult, error) {
	workDir, err := os.MkdirTemp("", "evidra-profile-*")
	if err != nil {
		return nil, fmt.Errorf("profile runner: create work dir: %w", err)
	}

	env := r.buildEnv(req, workDir)

	// Best-effort cleanup helper.
	doCleanup := func(ctx context.Context) {
		if req.Assets.CleanupScript != "" {
			// Ignore errors — this is best-effort.
			_ = r.runHook(ctx, req.Assets.CleanupScript, env)
		}
	}

	// Run install.sh.
	if err := r.runHook(ctx, req.Assets.InstallScript, env); err != nil {
		doCleanup(ctx)
		_ = os.RemoveAll(workDir)
		return nil, fmt.Errorf("profile runner: install.sh failed: %w", err)
	}

	// Run healthcheck.sh if present.
	if req.Assets.HealthcheckScript != "" {
		if err := r.runHook(ctx, req.Assets.HealthcheckScript, env); err != nil {
			doCleanup(ctx)
			_ = os.RemoveAll(workDir)
			return nil, fmt.Errorf("profile runner: healthcheck.sh failed: %w", err)
		}
	}

	// Parse lease.env if it exists.
	leaseEnv, err := parseLeaseEnv(filepath.Join(workDir, "lease.env"))
	if err != nil {
		doCleanup(ctx)
		_ = os.RemoveAll(workDir)
		return nil, fmt.Errorf("profile runner: parse lease.env: %w", err)
	}

	var releaseOnce sync.Once
	release := func(ctx context.Context) error {
		var releaseErr error
		releaseOnce.Do(func() {
			doCleanup(ctx)
			releaseErr = os.RemoveAll(workDir)
		})
		return releaseErr
	}

	return &ProfileRunResult{
		ExtraEnv: leaseEnv,
		Release:  release,
	}, nil
}

// buildEnv constructs the environment variable list for hook execution.
func (r *ProfileRunner) buildEnv(req ProfileRunRequest, workDir string) []string {
	env := []string{
		"KUBECONFIG=" + req.Kubeconfig,
		"EVIDRA_PROFILE=" + string(req.Profile),
		"EVIDRA_PROVIDER=" + req.Provider,
		"EVIDRA_CLUSTER_NAME=" + req.ClusterName,
		"EVIDRA_WORK_DIR=" + workDir,
		"EVIDRA_ASSETS_DIR=" + req.Assets.ProfileDir,
	}

	// Append extra env in sorted-key order for determinism is not needed;
	// the map iteration plus deterministic base env is sufficient for tests
	// that check specific keys.
	for k, v := range req.ExtraEnv {
		env = append(env, k+"="+v)
	}

	// Inherit PATH from the current process so hooks can find system binaries.
	if pathVal := os.Getenv("PATH"); pathVal != "" {
		env = append(env, "PATH="+pathVal)
	}

	return env
}

// runHook executes a shell script with the given environment.
func (r *ProfileRunner) runHook(ctx context.Context, script string, env []string) error {
	cmd := exec.CommandContext(ctx, "/bin/sh", script)
	cmd.Env = env
	cmd.Dir = filepath.Dir(script)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w\noutput: %s", filepath.Base(script), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// parseLeaseEnv reads a KEY=VALUE file, skipping empty lines and comments.
// Returns nil (not an error) if the file does not exist.
func parseLeaseEnv(path string) ([]string, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var result []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.Contains(line, "=") {
			continue
		}
		result = append(result, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
