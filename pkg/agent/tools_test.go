package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestValidateCommand_BlockedInteractive(t *testing.T) {
	t.Parallel()

	blocked := []string{
		"kubectl edit deployment/nginx -n bench",
		"kubectl exec -it nginx -- bash",
		"kubectl exec -ti nginx -- sh",
		"kubectl exec --stdin --tty nginx -- bash",
		"kubectl attach nginx",
		"kubectl port-forward svc/nginx 8080:80",
		"kubectl proxy",
		"kubectl run -it test --image=busybox -- sh",
		"terraform console",
	}

	for _, cmd := range blocked {
		if err := validateCommand(cmd); err == nil {
			t.Errorf("expected %q to be blocked", cmd)
		}
	}

	allowed := []string{
		"kubectl get pods -n bench",
		"kubectl apply -f manifest.yaml",
		"kubectl delete pod nginx -n bench",
		"kubectl describe deployment/nginx -n bench",
		"kubectl logs nginx -n bench",
		"kubectl rollout restart deployment/nginx -n bench",
		"kubectl patch configmap foo -n bench --type merge -p '{}'",
		"kubectl exec nginx -- cat /etc/nginx/nginx.conf",
		"kubectl set image deployment/nginx nginx=nginx:1.25",
		"helm install myrelease ./chart",
		"helm upgrade myrelease ./chart",
		"terraform apply -auto-approve",
		// Compound commands with cd.
		"cd /tmp && terraform plan",
		"cd /tmp && terraform apply -auto-approve",
		"cd dir && kubectl get pods",
		// File editing tools.
		"sed -i '' 's/old/new/' file.tf",
		"cp file.tf file.tf.bak",
		"tee file.tf",
		"mkdir -p modules/app",
	}

	for _, cmd := range allowed {
		if err := validateCommand(cmd); err != nil {
			t.Errorf("expected %q to be allowed, got: %v", cmd, err)
		}
	}
}

func TestValidateCommand_BlockedWatch(t *testing.T) {
	t.Parallel()

	blocked := []string{
		"kubectl get pods -w",
		"kubectl get pods --watch",
		"kubectl get pods -n bench -w",
		"kubectl get pods -n bench --watch",
		"kubectl get deployments -w -o wide",
	}

	for _, cmd := range blocked {
		if err := validateCommand(cmd); err == nil {
			t.Errorf("expected %q to be blocked (watch mode)", cmd)
		}
	}

	allowed := []string{
		"kubectl get pods -o wide",
		"kubectl get pods",
		"kubectl get pods -n bench",
	}

	for _, cmd := range allowed {
		if err := validateCommand(cmd); err != nil {
			t.Errorf("expected %q to be allowed, got: %v", cmd, err)
		}
	}
}

func TestValidateCommand_BlockedPipeBypass(t *testing.T) {
	t.Parallel()

	blocked := []string{
		"kubectl get pods | bash -c 'echo pwned'",
		"kubectl get pods | python -c 'print(1)'",
		"kubectl get pods | /bin/sh -c 'echo pwned'",
		"grep ok file | node -e 'console.log(1)'",
		"kubectl get pods & bash -c 'echo pwned'",
		"kubectl get pods\nbash -c 'echo pwned'",
	}

	for _, cmd := range blocked {
		if err := validateCommand(cmd); err == nil {
			t.Errorf("expected %q to be blocked", cmd)
		}
	}

	allowed := []string{
		"kubectl get pods | jq '.items | length'",
		"kubectl get pods | grep nginx | wc -l",
		"grep 'ready|running' pods.txt",
		"sed -E 's/foo|bar/baz/' file.txt",
	}

	for _, cmd := range allowed {
		if err := validateCommand(cmd); err != nil {
			t.Errorf("expected %q to be allowed, got: %v", cmd, err)
		}
	}
}

func TestValidateCommand_BlockedShellSubstitution(t *testing.T) {
	t.Parallel()

	blocked := []string{
		"kubectl get pods $(bash -c 'echo pwned')",
		"echo $(python -c 'print(1)')",
		"kubectl get pods `bash -c 'echo pwned'`",
		"kubectl get pods <(bash -c 'echo pwned')",
	}

	for _, cmd := range blocked {
		if err := validateCommand(cmd); err == nil {
			t.Errorf("expected %q to be blocked", cmd)
		}
	}

	allowed := []string{
		"echo '$(not command substitution)'",
		"grep '`literal`' file.txt",
		"echo ${KUBECONFIG}",
	}

	for _, cmd := range allowed {
		if err := validateCommand(cmd); err != nil {
			t.Errorf("expected %q to be allowed, got: %v", cmd, err)
		}
	}
}

func TestValidateCommand_BlockedFindExec(t *testing.T) {
	t.Parallel()

	blocked := []string{
		"find . -exec bash -c 'echo pwned' {} \\;",
		"find . -execdir sh -c 'echo pwned' {} \\;",
		"find . -ok bash -c 'echo pwned' {} \\;",
	}

	for _, cmd := range blocked {
		if err := validateCommand(cmd); err == nil {
			t.Errorf("expected %q to be blocked", cmd)
		}
	}

	allowed := []string{
		"find . -name '*.yaml' -print",
		"find /tmp -maxdepth 1 -type f",
	}

	for _, cmd := range allowed {
		if err := validateCommand(cmd); err != nil {
			t.Errorf("expected %q to be allowed, got: %v", cmd, err)
		}
	}
}

func TestContainsFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cmd  string
		flag string
		want bool
	}{
		{"kubectl get pods -w", "-w", true},
		{"kubectl get pods --watch", "--watch", true},
		{"kubectl get pods -n bench -w", "-w", true},
		{"kubectl get pods -o wide", "-w", false},
		{"kubectl get pods", "-w", false},
		{"kubectl get pods -o wide", "--watch", false},
	}

	for _, tt := range tests {
		got := containsFlag(tt.cmd, tt.flag)
		if got != tt.want {
			t.Errorf("containsFlag(%q, %q) = %v, want %v", tt.cmd, tt.flag, got, tt.want)
		}
	}
}

func TestToolExecutorRunCommandContextKillsChildProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process group cleanup is Unix-specific")
	}

	dir := t.TempDir()
	fifo := dir + "/blocked.fifo"
	if err := exec.Command("mkfifo", fifo).Run(); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}
	t.Cleanup(func() {
		_ = exec.Command("pkill", "-f", fifo).Run()
	})

	payload, err := json.Marshal(map[string]string{"command": "cat " + fifo})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan string, 1)
	go func() {
		done <- (&ToolExecutor{}).runCommand(ctx, string(payload))
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runCommand did not return after context timeout")
	}
}

func TestToolExecutorRunCommandHasPerToolTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process group cleanup is Unix-specific")
	}

	originalTimeout := toolCommandTimeout
	toolCommandTimeout = 50 * time.Millisecond
	t.Cleanup(func() {
		toolCommandTimeout = originalTimeout
	})

	dir := t.TempDir()
	fifo := dir + "/blocked.fifo"
	if err := exec.Command("mkfifo", fifo).Run(); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}
	t.Cleanup(func() {
		_ = exec.Command("pkill", "-f", fifo).Run()
	})

	payload, err := json.Marshal(map[string]string{"command": "cat " + fifo})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	done := make(chan string, 1)
	go func() {
		done <- (&ToolExecutor{}).runCommand(context.Background(), string(payload))
	}()

	select {
	case result := <-done:
		if !strings.Contains(result, "command timed out") {
			t.Fatalf("result = %q, want timeout message", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runCommand did not return after per-tool timeout")
	}
}

func TestToolExecutorRunCommandUsesWorkspaceDir(t *testing.T) {
	workspace := t.TempDir()
	marker := filepath.Join(workspace, "workspace-marker.txt")
	if err := os.WriteFile(marker, []byte("workspace"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	payload, err := json.Marshal(map[string]string{"command": "cat workspace-marker.txt"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result := (&ToolExecutor{WorkspaceDir: workspace}).runCommand(context.Background(), string(payload))
	if result != "workspace" {
		t.Fatalf("result = %q, want workspace marker from executor workspace", result)
	}
}

func TestToolExecutorRunCommandExportsWorkspaceEnv(t *testing.T) {
	workspace := t.TempDir()
	payload, err := json.Marshal(map[string]string{"command": "echo $INFRA_BENCH_WORKSPACE"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result := (&ToolExecutor{WorkspaceDir: workspace}).runCommand(context.Background(), string(payload))
	if result != workspace {
		t.Fatalf("INFRA_BENCH_WORKSPACE = %q, want %q", result, workspace)
	}
}

func TestToolExecutorWriteFileUsesWorkspaceDir(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(outside); err != nil {
		t.Fatalf("chdir outside: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Errorf("restore cwd: %v", err)
		}
	})

	payload, err := json.Marshal(map[string]string{
		"path":    "manifests/app.yaml",
		"content": "apiVersion: v1\n",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result := (&ToolExecutor{WorkspaceDir: workspace}).writeFile(context.Background(), string(payload))
	if strings.HasPrefix(result, "error:") {
		t.Fatalf("writeFile returned %q", result)
	}
	if _, err := os.Stat(filepath.Join(workspace, "manifests", "app.yaml")); err != nil {
		t.Fatalf("workspace file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "manifests", "app.yaml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("outside file err = %v, want not exist", err)
	}
}

func TestToolExecutorWriteFileRejectsWorkspaceEscape(t *testing.T) {
	workspace := t.TempDir()
	outsidePath := filepath.Join(filepath.Dir(workspace), "escape.txt")
	t.Cleanup(func() { _ = os.Remove(outsidePath) })

	payload, err := json.Marshal(map[string]string{
		"path":    "../escape.txt",
		"content": "outside",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result := (&ToolExecutor{WorkspaceDir: workspace}).writeFile(context.Background(), string(payload))
	if !strings.HasPrefix(result, "error:") {
		t.Fatalf("writeFile result = %q, want error", result)
	}
	if _, err := os.Stat(outsidePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("escape file err = %v, want not exist", err)
	}
}

func TestToolExecutorWriteFileRejectsEscapingSymlink(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(workspace, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	payload, err := json.Marshal(map[string]string{
		"path":    "link/escape.txt",
		"content": "outside",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result := (&ToolExecutor{WorkspaceDir: workspace}).writeFile(context.Background(), string(payload))
	if !strings.HasPrefix(result, "error:") {
		t.Fatalf("writeFile result = %q, want error", result)
	}
	if _, err := os.Stat(filepath.Join(outside, "escape.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("escape file err = %v, want not exist", err)
	}
}

func TestRunProcessGroupCommandReturnsOnContextTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process group cleanup is Unix-specific")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	cmd := exec.Command("bash", "-c", "sleep 30")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	go func() {
		done <- runProcessGroupCommand(ctx, cmd)
	}()

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error = %v, want context deadline exceeded", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runProcessGroupCommand did not return after context timeout")
	}
}
