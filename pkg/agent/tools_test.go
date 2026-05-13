package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
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
