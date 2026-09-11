package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vitas/evidra-bench/pkg/evaluation"
)

func completedTestResult(verdict evaluation.Verdict) evaluation.Result {
	result := evaluation.Result{
		Version:     evaluation.ResultVersion,
		ID:          "test-evaluation",
		Suite:       evaluation.SuitePlan{ID: "kubernetes-demo@1"},
		Target:      evaluation.TargetPlan{Kind: evaluation.TargetModel, Provider: "openai", Model: "test"},
		StartedAt:   time.Unix(0, 0),
		EndedAt:     time.Unix(5, 0),
		Termination: evaluation.Termination{Kind: evaluation.TerminationComplete},
		Cleanup:     evaluation.CleanupResult{Attempted: true, Succeeded: true},
		Cases:       []evaluation.CaseResult{{ScenarioID: "repair", Verdict: verdict, Termination: evaluation.Termination{Kind: evaluation.TerminationComplete}}},
	}
	result.Summary = evaluation.Summarize(result)
	return result
}

func TestTestCommand_UsesSimpleDefaultsAndWritesCanonicalReports(t *testing.T) {
	outputDir := t.TempDir()
	var captured testRequest
	run := func(_ context.Context, req testRequest) (evaluation.Result, error) {
		captured = req
		return completedTestResult(evaluation.VerdictPass), nil
	}
	cmd := newTestCommand(run)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--model", "openai/gpt-test", "--output", outputDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("test command failed: %v", err)
	}
	if captured.Suite != "kubernetes-demo@1" || captured.Environment != "kind" {
		t.Fatalf("defaults = suite %q environment %q", captured.Suite, captured.Environment)
	}
	if captured.ProjectRoot != "" {
		t.Fatalf("internal project root default = %q, want automatic asset discovery", captured.ProjectRoot)
	}
	for _, name := range []string{"result.json", "report.html"} {
		if _, err := os.Stat(filepath.Join(outputDir, name)); err != nil {
			t.Fatalf("expected %s: %v", name, err)
		}
	}
	if !strings.Contains(stdout.String(), "PASS") || !strings.Contains(stdout.String(), "Report:") {
		t.Fatalf("unexpected terminal output:\n%s", stdout.String())
	}
}

func TestTestCommand_RejectsMissingTargetBeforeRunner(t *testing.T) {
	called := false
	cmd := newTestCommand(func(context.Context, testRequest) (evaluation.Result, error) {
		called = true
		return evaluation.Result{}, nil
	})
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--agent-image") {
		t.Fatalf("error = %v, want actionable missing target error naming the sandbox pair", err)
	}
	if called {
		t.Fatal("runner called before target validation")
	}
}

func TestTestCommandDoesNotExposeProjectRoot(t *testing.T) {
	cmd := newTestCommand(func(context.Context, testRequest) (evaluation.Result, error) {
		return completedTestResult(evaluation.VerdictPass), nil
	})
	if flag := cmd.Flags().Lookup("project-root"); flag != nil {
		t.Fatalf("test command exposes internal flag --%s", flag.Name)
	}
}

func TestTestCommand_MapsBehavioralAndIncompleteExitCodes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result evaluation.Result
		runErr error
		want   int
	}{
		{name: "unsafe", result: completedTestResult(evaluation.VerdictUnsafe), want: 1},
		{name: "incomplete", result: evaluation.Result{Version: evaluation.ResultVersion, Termination: evaluation.Termination{Kind: evaluation.TerminationIncomplete}}, runErr: errors.New("setup failed"), want: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newTestCommand(func(context.Context, testRequest) (evaluation.Result, error) { return tc.result, tc.runErr })
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs([]string{"--model", "openai/gpt-test", "--output", t.TempDir()})
			err := cmd.Execute()
			if got := exitCodeForError(err); got != tc.want {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, tc.want, err)
			}
		})
	}
}

func TestTestCommandSandboxPairValidation(t *testing.T) {
	notCalled := func(t *testing.T, args ...string) string {
		t.Helper()
		called := false
		cmd := newTestCommand(func(context.Context, testRequest) (evaluation.Result, error) {
			called = true
			return evaluation.Result{}, nil
		})
		cmd.SetArgs(args)
		err := cmd.Execute()
		if called {
			t.Fatal("runner must not run with invalid sandbox pairing")
		}
		if err == nil {
			t.Fatalf("expected rejection for %v", args)
		}
		return err.Error()
	}
	if e := notCalled(t, "--agent-image", "img", "--environment", "kind"); !strings.Contains(e, "together") {
		t.Fatalf("half pair: %v", e)
	}
	if e := notCalled(t, "--model", "m", "--agent-image", "i", "--agent-bundle", "b", "--environment", "kind"); !strings.Contains(e, "mutually exclusive") {
		t.Fatalf("model+sandbox: %v", e)
	}
	if e := notCalled(t, "--agent", "a", "--agent-image", "i", "--agent-bundle", "b", "--environment", "kind"); !strings.Contains(e, "bundle IS the agent") {
		t.Fatalf("agent+sandbox: %v", e)
	}

	// Full pair alone is a complete target: reaches the runner.
	seen := testRequest{}
	cmd := newTestCommand(func(_ context.Context, req testRequest) (evaluation.Result, error) {
		seen = req
		res := completedTestResult(evaluation.VerdictPass)
		res.Target = evaluation.TargetPlan{Kind: evaluation.TargetAgent}
		return res, nil
	})
	out := t.TempDir()
	cmd.SetArgs([]string{"--agent-image", "img", "--agent-bundle", "/b", "--environment", "kind", "--output", out})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pair must validate: %v", err)
	}
	if seen.AgentImage != "img" || seen.AgentBundleDir != "/b" {
		t.Fatalf("request fields lost: %+v", seen)
	}
}
