package environment

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Agent sandbox (ADR 0001 Phase 6). Qualified runs execute their agent in a
// hardened sibling container attached to the cluster network — never in the
// runner process, and never with runner credentials. DooD constraint (same
// as audit staging): the daemon cannot see this container's filesystem, so
// every input crosses into the sandbox through a NAMED VOLUME staged with a
// tar pipe; bind mounts of host paths from inside the runner are not
// possible.
//
// The mandatory hardening profile is asserted structurally here (flags are
// literal, no caller can relax them):
//
//	--network <cluster network>  (nothing else can reach the API server)
//	--read-only                  (root fs immutable)
//	--cap-drop ALL --security-opt no-new-privileges
//	--user 65534:65534           (nobody; volume staged world-readable)
//	--tmpfs /tmp                 (only writable paths: /tmp and /workspace)
//	--memory / --cpus            (bounded)
//	-e only the declared allow-list (no runner env inheritance)

// ErrSandboxUnavailable marks a run whose required sandbox could not be
// provided; the harness maps it to INCOMPLETE (sandbox_unavailable) — the
// run is never silently downgraded to unconfined execution.
var envNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var ErrSandboxUnavailable = errors.New("sandbox unavailable")

// SandboxSpec describes one sandboxed agent execution.
type SandboxSpec struct {
	RunID         string
	Image         string            // caller-supplied agent image
	BundleDir     string            // host dir: entrypoint "run" + declared files
	Entrypoint    []string          // exec argv inside the sandbox (default /agent/run)
	PromptContent string            // run prompt, staged as /agent/prompt.md
	Kubeconfig    string            // per-run identity kubeconfig (host-visible path content read now)
	ExtraFiles    map[string]string // staged name -> content (whitelisted inputs)
	// AgentEnv passes EXPLICIT named env vars into the sandbox (no runner
	// inheritance — this map is the whole channel; the harness sends e.g.
	// INFRA_BENCH_SCENARIO, which fixture agents legitimately need).
	AgentEnv map[string]string
	Network  string // cluster docker network name
	Memory   string // e.g. "512m"; empty = docker default
	CPUs     string // e.g. "1.0"; empty = unlimited-ish
	Timeout  time.Duration
}

// SandboxRunner executes sandbox commands and reports the result.
type SandboxRunner interface {
	// Available probes whether a sandbox can be provided at all; failure
	// must map to ErrSandboxUnavailable (INCOMPLETE), never unconfined
	// fallback.
	Available(ctx context.Context) error
	Run(ctx context.Context, spec SandboxSpec, argv []string) (*SandboxRun, error)
}

// SandboxRun is the captured outcome of one sandboxed exec.
type SandboxRun struct {
	Stdout   string
	Stderr   string
	ExitCode int // -1 = could not start / killed
}

// DockerSandbox implements SandboxRunner through the docker CLI (DooD).
type DockerSandbox struct {
	// StageImage is a tiny trusted image used to populate the input volume
	// before the agent image starts (alpine:3.22 — the proven recipe).
	StageImage string
	// Volumes keeps created volume names for cleanup.
	created []string
}

// SandboxVolumeName is the named volume carrying the agent's staged inputs.
func SandboxVolumeName(runID string) string {
	if len(runID) > 40 {
		runID = runID[:40]
	}
	return "evidra-agent-" + strings.ToLower(runID)
}

// available probes the docker daemon (socket present in this container).
func (s *DockerSandbox) Available(ctx context.Context) error {
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, "docker", "version", "--format", "{{.Server.Version}}")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: docker daemon unreachable: %v: %s", ErrSandboxUnavailable, err, truncate(string(out), 200))
	}
	return nil
}

// buildTar packs bundle files + prompt + kubeconfig + extras into a tar
// stream rooted at the volume (agent/ prefix).
func (s *DockerSandbox) buildTar(spec SandboxSpec) ([]byte, error) {
	buf := &bytes.Buffer{}
	tw := tar.NewWriter(buf)
	write := func(name string, mode int64, data []byte) error {
		hdr := &tar.Header{Name: name, Mode: mode, Size: int64(len(data)), Format: tar.FormatPAX}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err := tw.Write(data)
		return err
	}
	if spec.BundleDir != "" {
		err := filepath.Walk(spec.BundleDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			rel, rerr := filepath.Rel(spec.BundleDir, path)
			if rerr != nil {
				return rerr
			}
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			mode := int64(0o644)
			if info.Mode()&0o111 != 0 {
				mode = 0o755
			}
			return write("agent/"+filepath.ToSlash(rel), mode, data)
		})
		if err != nil {
			return nil, fmt.Errorf("sandbox: bundle: %w", err)
		}
	}
	if spec.PromptContent != "" {
		if err := write("agent/prompt.md", 0o644, []byte(spec.PromptContent)); err != nil {
			return nil, err
		}
	}
	for name, content := range spec.ExtraFiles {
		if err := write("inputs/"+filepath.ToSlash(name), 0o644, []byte(content)); err != nil {
			return nil, err
		}
	}
	if spec.Kubeconfig != "" {
		data, err := os.ReadFile(spec.Kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("sandbox: kubeconfig: %w", err)
		}
		if err := write("run/agent.kubeconfig", 0o644, data); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Run starts the sandbox container detached, execs argv inside, captures
// output, and always removes the container + staging volume.
func (s *DockerSandbox) Run(ctx context.Context, spec SandboxSpec, argv []string) (*SandboxRun, error) {
	if spec.RunID == "" || spec.Image == "" || spec.Network == "" {
		return nil, fmt.Errorf("sandbox: RunID, Image and Network are required")
	}
	vol := SandboxVolumeName(spec.RunID)
	if err := s.stage(ctx, spec, vol); err != nil {
		s.cleanup(ctx, vol, "")
		return nil, err
	}
	name := "evidra-sbx-" + strings.ToLower(spec.RunID)
	args := append([]string{"run", "-d", "--name", name,
		"--network", spec.Network,
		"--read-only",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--user", "65534:65534",
		"--tmpfs", "/tmp:rw,size=64m",
		"-v", vol + ":/mnt/evidra:ro",
	}, resourceFlags(spec)...)
	args = append(args, "-e", "KUBECONFIG=/mnt/evidra/run/agent.kubeconfig", "-e", "HOME=/tmp")
	{
		keys := make([]string, 0, len(spec.AgentEnv))
		for k := range spec.AgentEnv {
			if !envNameRe.MatchString(k) {
				continue // refuse junk names, never silently pass weird keys
			}
			switch k {
			case "KUBECONFIG", "HOME", "PATH":
				continue // reserved: the sandbox identity contract, never overridable
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			args = append(args, "-e", k+"="+spec.AgentEnv[k])
		}
	}
	// The container is a parking lot, NOT the agent: its argv must survive
	// arbitrary ENTRYPOINTs (the bench image ships ENTRYPOINT [evidra],
	// which ate `sleep` and killed the sandbox in the Phase 9 matrix).
	// `sh` is the only image requirement; a missing sh fails startup
	// honestly as sandbox-unavailable rather than misattributing.
	args = append(args,
		"--entrypoint", "", spec.Image, "sh", "-c", "sleep 2147483647")
	if out, err := s.docker(ctx, args...); err != nil {
		s.cleanup(ctx, vol, name)
		return nil, fmt.Errorf("%w: start %s: %v: %s", ErrSandboxUnavailable, spec.Image, err, truncate(string(out), 300))
	}
	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	rctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	res := &SandboxRun{ExitCode: -1}
	//nolint:gosec // argv is scenario/caller-supplied command contract.
	ex := exec.CommandContext(rctx, "docker", append([]string{"exec", name}, argv...)...)
	var sout, serr bytes.Buffer
	ex.Stdout, ex.Stderr = &sout, &serr
	runErr := ex.Run()
	res.Stdout, res.Stderr = sout.String(), serr.String()
	if ex.ProcessState != nil {
		res.ExitCode = ex.ProcessState.ExitCode()
		if rctx.Err() != nil && res.ExitCode != 0 {
			res.Stderr += "\nsandbox: agent exceeded timeout " + timeout.String()
		}
	} else if runErr != nil {
		res.Stderr += "\nsandbox: could not exec agent: " + runErr.Error()
	}
	s.cleanup(ctx, vol, name)
	return res, nil
}

func resourceFlags(spec SandboxSpec) []string {
	var out []string
	if spec.Memory != "" {
		out = append(out, "--memory", spec.Memory)
	}
	if spec.CPUs != "" {
		out = append(out, "--cpus", spec.CPUs)
	}
	return out
}

func (s *DockerSandbox) stage(ctx context.Context, spec SandboxSpec, vol string) error {
	if _, err := s.docker(ctx, "volume", "create", vol); err != nil {
		return fmt.Errorf("%w: volume create: %v", ErrSandboxUnavailable, err)
	}
	s.created = append(s.created, vol)
	data, err := s.buildTar(spec)
	if err != nil {
		return err
	}
	img := s.StageImage
	if img == "" {
		img = "alpine:3.22"
	}
	cctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	//nolint:gosec // fixed recipe; tar content built from whitelisted inputs.
	cmd := exec.CommandContext(cctx, "docker", "run", "--rm", "-i", "-v", vol+":/mnt/evidra", img,
		"sh", "-c", "rm -rf /mnt/evidra/* && tar x -C /mnt/evidra")
	cmd.Stdin = bytes.NewReader(data)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sandbox: stage volume: %v: %s", err, truncate(string(out), 300))
	}
	return nil
}

func (s *DockerSandbox) cleanup(ctx context.Context, vol, container string) {
	if container != "" {
		_, _ = s.docker(context.WithoutCancel(ctx), "rm", "-f", container)
	}
	if vol != "" {
		_, _ = s.docker(context.WithoutCancel(ctx), "volume", "rm", vol)
	}
}

func (s *DockerSandbox) docker(ctx context.Context, args ...string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	//nolint:gosec // docker CLI wrapper, args fixed by caller.
	out, err := exec.CommandContext(cctx, "docker", args...).CombinedOutput()
	return string(out), err
}

// (No ClusterNetworkName string-builder: kind shares the "kind" network
// across clusters — DockerNetworkOf(inspect) is the only correct answer.)

// DockerNetworkOf returns the docker network a container is attached to
// (first one). Empirically required: kind clusters share the "kind"
// network regardless of cluster name, so string-building the network name
// from the cluster name is simply wrong; inspecting a node never is.
// Empty string on any failure (=> sandbox unavailable, honestly).
func DockerNetworkOf(container string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	//nolint:gosec // fixed argv.
	cmd := exec.CommandContext(ctx, "docker", "inspect", "-f",
		"{{range $k, $_ := .NetworkSettings.Networks}}{{$k}}{{end}}", container)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
