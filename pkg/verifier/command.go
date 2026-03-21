package verifier

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CommandSucceedsCheck runs a script and passes if it exits 0.
type CommandSucceedsCheck struct {
	Name    string
	Command string // path to script or command to run
}

// Validate checks that required fields are set.
func (c *CommandSucceedsCheck) Validate() error {
	if c.Command == "" {
		return fmt.Errorf("verifier.CommandSucceedsCheck: command (condition) is required")
	}
	return nil
}

// Check runs the command and returns pass if it exits 0.
func (c *CommandSucceedsCheck) Check(ctx context.Context, kubeconfigPath string) CheckResult {
	name := fmt.Sprintf("command-succeeds/%s", c.Name)
	cmd := exec.CommandContext(ctx, "bash", "-c", c.Command)
	cmd.Env = append(cmd.Env,
		fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath),
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
		fmt.Sprintf("HOME=%s", os.Getenv("HOME")),
	)
	// Inherit AWS env vars if set.
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "AWS_") {
			cmd.Env = append(cmd.Env, env)
		}
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return CheckResult{Name: name, Type: "command-succeeds", Verdict: VerdictFail, Message: string(out)}
	}
	return CheckResult{Name: name, Type: "command-succeeds", Verdict: VerdictPass, Message: string(out)}
}
