package harness

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"samebits.com/evidra-infra-bench/pkg/adapter"
	"samebits.com/evidra-infra-bench/pkg/environment"
)

// fakeProvider is a test double for environment.ClusterLifecycle.
type fakeProvider struct {
	created   bool
	destroyed bool
}

func (f *fakeProvider) Create(_ context.Context, clusterName string, _ environment.ClusterSpec) (*environment.Handle, error) {
	f.created = true
	return &environment.Handle{
		ClusterName:    clusterName,
		KubeconfigPath: "/tmp/fake-kubeconfig",
	}, nil
}

func (f *fakeProvider) Destroy(_ context.Context, _ *environment.Handle) error {
	f.destroyed = true
	return nil
}

func (f *fakeProvider) Recreate(_ context.Context, clusterName string, _ environment.ClusterSpec) (*environment.Handle, error) {
	return &environment.Handle{ClusterName: clusterName, KubeconfigPath: "/tmp/fake-kubeconfig"}, nil
}

func (f *fakeProvider) HealthCheck(_ context.Context, _ string) error { return nil }

func (f *fakeProvider) ForceDeleteNamespace(_ context.Context, _, _ string) error { return nil }

func (f *fakeProvider) CreateNamespace(_ context.Context, _, _ string) error { return nil }

func (f *fakeProvider) RunCanary(_ context.Context, _, _ string) error { return nil }

// fakeAdapter is a test double for adapter.Adapter.
type fakeAdapter struct {
	called bool
}

func (f *fakeAdapter) Run(_ context.Context, _ adapter.RunInput) (*adapter.RunResult, error) {
	f.called = true
	return &adapter.RunResult{
		ExitCode:   0,
		Stdout:     "agent did things",
		Transcript: "agent did things",
		Metadata:   map[string]string{"adapter": "fake"},
	}, nil
}

type sleepingAdapter struct {
	delay time.Duration
}

func (s *sleepingAdapter) Run(ctx context.Context, _ adapter.RunInput) (*adapter.RunResult, error) {
	timer := time.NewTimer(s.delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return &adapter.RunResult{ExitCode: 0, Transcript: "done"}, nil
	}
}

type autopsyAdapter struct{}

func (a *autopsyAdapter) Run(_ context.Context, _ adapter.RunInput) (*adapter.RunResult, error) {
	return &adapter.RunResult{
		ExitCode:   0,
		Transcript: "The deployment is fixed and everything is working.",
		ToolCalls: []adapter.ToolCallRecord{
			{
				Tool:   "run_command",
				Args:   map[string]any{"command": "kubectl get pods -n bench"},
				Result: "web 0/1 ErrImagePull",
			},
			{
				Tool:   "run_command",
				Args:   map[string]any{"command": "kubectl get pods -n bench"},
				Result: "web 0/1 ErrImagePull",
			},
			{
				Tool:   "run_command",
				Args:   map[string]any{"command": "kubectl get pods -n bench"},
				Result: "web 0/1 ErrImagePull",
			},
		},
		Metadata: map[string]string{"turns": "3", "prompt_tokens": "100", "completion_tokens": "50"},
	}, nil
}

type workspaceCheckingAdapter struct {
	exists bool
}

func (w *workspaceCheckingAdapter) Run(_ context.Context, input adapter.RunInput) (*adapter.RunResult, error) {
	info, err := os.Stat(input.WorkspaceDir)
	if err == nil && info.IsDir() {
		w.exists = true
	}
	return &adapter.RunResult{ExitCode: 0}, nil
}

type fakeRunner struct {
	err error
}

func (f *fakeRunner) Run(_ context.Context, _ *exec.Cmd) ([]byte, error) {
	return nil, f.err
}

type recordingRunner struct {
	commands []string
}

func (r *recordingRunner) Run(_ context.Context, cmd *exec.Cmd) ([]byte, error) {
	r.commands = append(r.commands, strings.Join(cmd.Args, " "))
	return nil, nil
}

func fakeDeps() (Deps, *fakeProvider, *fakeAdapter) {
	fp := &fakeProvider{}
	fa := &fakeAdapter{}
	return Deps{
		EnvProvider: fp,
		Adapter:     fa,
	}, fp, fa
}

// fakeKubeconfig creates a temporary kubeconfig file and returns its path.
// The file must exist on disk for the harness to pass its stat check.
func fakeKubeconfig(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "kubeconfig-*")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func writePromptMetadataFile(t *testing.T, contractVersion, promptVersion string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "prompt.md")
	body := "<!-- contract: " + contractVersion + " -->\n" +
		"<!-- prompt: " + promptVersion + " -->\n" +
		"You are an infra agent.\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
