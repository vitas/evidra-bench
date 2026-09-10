package verifier

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// CommandSucceedsCheck runs a script and judges it by exit code.
//
// Legacy policy (kept only for third-party scenarios; starter suites use
// assert-v2): exit 0 = pass, exit 1 = fail, exit >= 2 (or a process that
// could not run) = error. Exit 1-vs-failure ambiguity is exactly why the
// structured protocol exists.
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

// Check runs the command and applies the legacy exit-code policy.
func (c *CommandSucceedsCheck) Check(ctx context.Context, kubeconfigPath string) CheckResult {
	name := fmt.Sprintf("command-succeeds/%s", c.Name)
	out, exitCode, err := runScript(ctx, c.Command, kubeconfigPath)
	switch exitCode {
	case 0:
		return CheckResult{Name: name, Type: "command-succeeds", Verdict: VerdictPass, Message: string(out)}
	case 1:
		return CheckResult{Name: name, Type: "command-succeeds", Verdict: VerdictFail, Message: string(out)}
	default:
		detail := strings.TrimSpace(string(out))
		if exitCode < 0 {
			detail = firstNonEmptyString(detail, errText(err))
			return CheckResult{Name: name, Type: "command-succeeds", Verdict: VerdictError,
				Message: detail, Error: &CheckError{Kind: ErrorKindTransport, Message: "verifier command could not run: " + detail}}
		}
		return CheckResult{Name: name, Type: "command-succeeds", Verdict: VerdictError,
			Message: detail, Error: &CheckError{Kind: ErrorKindExitCode, Message: fmt.Sprintf("verifier exited %d: %s", exitCode, detail)}}
	}
}

// AssertV2Check runs a script speaking the structured protocol (see
// protocol.go). The parsed status is authoritative; the exit code is
// ignored once the document parses, so wrappers cannot fake outcomes.
type AssertV2Check struct {
	Name    string
	Command string
}

// Validate checks required fields.
func (c *AssertV2Check) Validate() error {
	if c.Command == "" {
		return fmt.Errorf("verifier.AssertV2Check: command (condition) is required")
	}
	return nil
}

// Check runs the assert-v2 script and classifies per design §2.
func (c *AssertV2Check) Check(ctx context.Context, kubeconfigPath string) CheckResult {
	name := fmt.Sprintf("assert-v2/%s", c.Name)
	out, exitCode, err := runScript(ctx, c.Command, kubeconfigPath)
	if ctx.Err() != nil && exitCode < 0 {
		return errorResult(name, c.Type(), VerdictError, &CheckError{Kind: ErrorKindTimeout, Message: "assert-v2 script timed out"}, truncateTail(string(out), 2048))
	}
	doc, ok := ParseProtocolOutput(out)
	if !ok {
		kind := ErrorKindParse
		msg := "no parseable protocol document on stdout"
		if exitCode < 0 || exitCode == 127 {
			// 127 = bash "command not found": the script itself never ran.
			kind = ErrorKindTransport
			msg = "assert-v2 script could not run (exit " + fmt.Sprint(exitCode) + "): " + errText(err)
		}
		return errorResult(name, "assert-v2", VerdictError,
			&CheckError{Kind: kind, Message: msg}, truncateTail(string(out), 2048))
	}
	switch doc.Status {
	case ProtocolStatusError:
		return errorResult(name, "assert-v2", VerdictError,
			&CheckError{Kind: doc.Error.Kind, Message: doc.Error.Message}, "")
	case ProtocolStatusPass:
		return CheckResult{Name: name, Type: "assert-v2", Verdict: VerdictPass, Message: renderAssertions(doc)}
	default: // fail
		return CheckResult{Name: name, Type: "assert-v2", Verdict: VerdictFail, Message: renderAssertions(doc)}
	}
}

func (c *AssertV2Check) Type() string { return "assert-v2" }

func errorResult(name, typ string, v Verdict, e *CheckError, detail string) CheckResult {
	if detail != "" {
		e.Message = strings.TrimSpace(e.Message + "\n" + detail)
	}
	return CheckResult{Name: name, Type: typ, Verdict: v, Message: e.Message, Error: e}
}

func renderAssertions(doc ProtocolDocument) string {
	parts := make([]string, 0, len(doc.Assertions))
	for _, a := range doc.Assertions {
		status := "PASS"
		if !a.Passed {
			status = "FAIL"
		}
		line := fmt.Sprintf("%s %s", status, a.Name)
		if a.Observed != "" {
			line += " (observed: " + truncateTail(a.Observed, 200) + ")"
		}
		parts = append(parts, line)
	}
	return strings.Join(parts, "\n")
}

// runScript executes command via bash with the standard verifier
// environment and reports (combined output, exit code, exec error).
// exitCode is -1 when the process could not be started at all.
func runScript(ctx context.Context, command, kubeconfigPath string) ([]byte, int, error) {
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Env = append(cmd.Env,
		fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath),
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
		fmt.Sprintf("HOME=%s", os.Getenv("HOME")),
	)
	// Inherit AWS env vars if set (legacy behavior kept verbatim).
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "AWS_") {
			cmd.Env = append(cmd.Env, env)
		}
	}
	out, err := cmd.CombinedOutput()
	if err == nil {
		return out, 0, nil
	}
	var exitErr *exec.ExitError
	if ok := asExitError(err, &exitErr); ok {
		code := exitErr.ExitCode()
		if code == -1 {
			// Killed by signal (e.g. context timeout): report transport;
			// callers inspect ctx.Err() to refine to timeout.
			code = -1
		}
		return out, code, err
	}
	return out, -1, err
}

func asExitError(err error, target **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*target = ee
		return true
	}
	return false
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	if pe, ok := err.(*os.PathError); ok { //nolint:staticcheck
		return pe.Error()
	}
	if ee, ok := err.(*exec.ExitError); ok {
		if ws, ok2 := ee.Sys().(syscall.WaitStatus); ok2 && ws.Signaled() {
			return "signal: " + ws.Signal().String()
		}
	}
	return err.Error()
}

func truncateTail(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return "…" + s[len(s)-max:]
}

func firstNonEmptyString(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
