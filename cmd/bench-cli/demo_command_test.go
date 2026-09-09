package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/vitas/evidra-bench/pkg/evaluation"
	"github.com/vitas/evidra-bench/pkg/modelconfig"
)

type demoRunnerSpy struct {
	requests []testRequest
	result   evaluation.Result
	err      error
}

func (s *demoRunnerSpy) run(_ context.Context, req testRequest) (evaluation.Result, error) {
	s.requests = append(s.requests, req)
	return s.result, s.err
}

func demoResult() evaluation.Result {
	return completedTestResult(evaluation.VerdictPass)
}

func executeDemo(t *testing.T, spy *demoRunnerSpy, discover demoDiscover, stdin string, args ...string) (string, error) {
	t.Helper()
	cmd := newDemoCommand(spy.run, discover, func(io.Reader) bool { return true }, func(context.Context, testRequest) ([]string, error) {
		return []string{"tools"}, nil
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestDemoDiscoveryUsesCapabilitiesResolvedFromSuite(t *testing.T) {
	spy := &demoRunnerSpy{result: demoResult()}
	var discoveredWith []string
	cmd := newDemoCommand(spy.run, func(_ context.Context, required []string) ([]modelconfig.LocalModel, error) {
		discoveredWith = append([]string(nil), required...)
		return []modelconfig.LocalModel{{Name: "model:1"}}, nil
	}, func(io.Reader) bool { return true }, func(_ context.Context, req testRequest) ([]string, error) {
		if req.Suite != "kubernetes-demo@1" {
			t.Fatalf("suite = %q", req.Suite)
		}
		return []string{"suite-owned-capability"}, nil
	})
	cmd.SetArgs([]string{"--output", t.TempDir()})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(discoveredWith, ",") != "suite-owned-capability" {
		t.Fatalf("discovery capabilities = %v", discoveredWith)
	}
}

func TestDemoCommandExplicitOllamaModelUsesSharedRunner(t *testing.T) {
	spy := &demoRunnerSpy{result: demoResult()}
	out, err := executeDemo(t, spy, func(context.Context, []string) ([]modelconfig.LocalModel, error) {
		t.Fatal("explicit model must skip discovery")
		return nil, nil
	}, "", "--model", "ollama/qwen3:8b", "--output", t.TempDir())
	if err != nil {
		t.Fatalf("demo failed: %v\n%s", err, out)
	}
	if len(spy.requests) != 1 {
		t.Fatalf("runner calls = %d", len(spy.requests))
	}
	req := spy.requests[0]
	if req.Model != "ollama/qwen3:8b" || req.Suite != "kubernetes-demo@1" || req.Environment != "kind" {
		t.Fatalf("request = %+v", req)
	}
	if req.Agent != "" || req.Endpoint != "" {
		t.Fatalf("demo invented endpoint/agent fields: %+v", req)
	}
}

func TestDemoCommandBareModelNameMapsToOllama(t *testing.T) {
	spy := &demoRunnerSpy{result: demoResult()}
	if _, err := executeDemo(t, spy, nil, "", "--model", "qwen3:8b", "--output", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if got := spy.requests[0].Model; got != "ollama/qwen3:8b" {
		t.Fatalf("model = %q, want ollama/qwen3:8b", got)
	}
}

func TestDemoCommandAutoSelectsOnlyCompatibleModel(t *testing.T) {
	spy := &demoRunnerSpy{result: demoResult()}
	var required []string
	out, err := executeDemo(t, spy, func(_ context.Context, want []string) ([]modelconfig.LocalModel, error) {
		required = want
		return []modelconfig.LocalModel{{Name: "qwen3:8b", ParameterSize: "8.4B"}}, nil
	}, "", "--output", t.TempDir())
	if err != nil {
		t.Fatalf("demo failed: %v\n%s", err, out)
	}
	if strings.Join(required, ",") != "tools" {
		t.Fatalf("discovery required capabilities = %v, want [tools]", required)
	}
	if spy.requests[0].Model != "ollama/qwen3:8b" {
		t.Fatalf("request = %+v", spy.requests[0])
	}
}

func TestDemoCommandZeroModelsGivesGuidanceWithoutPulling(t *testing.T) {
	spy := &demoRunnerSpy{}
	out, err := executeDemo(t, spy, func(context.Context, []string) ([]modelconfig.LocalModel, error) {
		return nil, nil
	}, "", "--output", t.TempDir())
	if err == nil {
		t.Fatal("expected guidance error")
	}
	if !strings.Contains(out, "ollama pull") || !strings.Contains(out, "--model") {
		t.Fatalf("output must guide install-or-explicit without pulling: %s", out)
	}
	if code := exitCodeForError(err); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if len(spy.requests) != 0 {
		t.Fatal("runner must not run without a model")
	}
}

func TestDemoCommandPromptsOnceForMultipleModels(t *testing.T) {
	spy := &demoRunnerSpy{result: demoResult()}
	out, err := executeDemo(t, spy, func(context.Context, []string) ([]modelconfig.LocalModel, error) {
		return []modelconfig.LocalModel{
			{Name: "qwen3:8b"},
			{Name: "llama3.1:70b"},
		}, nil
	}, "2\n", "--output", t.TempDir())
	if err != nil {
		t.Fatalf("demo failed: %v\n%s", err, out)
	}
	if strings.Count(out, "Select a local model") != 1 {
		t.Fatalf("expected exactly one prompt: %s", out)
	}
	if spy.requests[0].Model != "ollama/llama3.1:70b" {
		t.Fatalf("request = %+v, want second choice", spy.requests[0])
	}
}

func TestDemoCommandInvalidSelectionFails(t *testing.T) {
	spy := &demoRunnerSpy{}
	_, err := executeDemo(t, spy, func(context.Context, []string) ([]modelconfig.LocalModel, error) {
		return []modelconfig.LocalModel{{Name: "a:1"}, {Name: "b:2"}}, nil
	}, "9\n", "--output", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "selection") {
		t.Fatalf("err = %v", err)
	}
	if len(spy.requests) != 0 {
		t.Fatal("invalid selection must not run the evaluation")
	}
}

func TestDemoCommandRequiresExplicitModelInCI(t *testing.T) {
	spy := &demoRunnerSpy{}
	prompted := false
	_, err := executeDemo(t, spy, func(context.Context, []string) ([]modelconfig.LocalModel, error) {
		prompted = true
		return []modelconfig.LocalModel{{Name: "a:1"}, {Name: "b:2"}}, nil
	}, "", "--ci", "--output", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "--model") {
		t.Fatalf("err = %v, want --model requirement", err)
	}
	if prompted {
		t.Fatal("CI mode must not probe interactively")
	}
}

func TestChooseDemoModelRejectsNonInteractiveSelectionBeforeDiscovery(t *testing.T) {
	discovered := false
	spy := &demoRunnerSpy{}
	cmd := newDemoCommand(spy.run, func(context.Context, []string) ([]modelconfig.LocalModel, error) {
		discovered = true
		return []modelconfig.LocalModel{{Name: "a:1"}, {Name: "b:2"}}, nil
	}, func(io.Reader) bool { return false }, func(context.Context, testRequest) ([]string, error) {
		return []string{"tools"}, nil
	})
	cmd.SetArgs([]string{"--output", t.TempDir()})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "non-interactive") || !strings.Contains(err.Error(), "--model") {
		t.Fatalf("error = %v, want non-interactive --model guidance", err)
	}
	if discovered {
		t.Fatal("non-interactive demo without --model must not perform discovery")
	}
}

func TestDemoCommandUnreachableRuntimeNamesTheBoundary(t *testing.T) {
	spy := &demoRunnerSpy{}
	_, err := executeDemo(t, spy, func(context.Context, []string) ([]modelconfig.LocalModel, error) {
		return nil, errors.New("ollama runtime unreachable: connection refused")
	}, "", "--output", t.TempDir())
	if err == nil {
		t.Fatal("expected error")
	}
	text := err.Error()
	if !strings.Contains(text, "unreachable") || !strings.Contains(text, "ollama serve") {
		t.Fatalf("error must name boundary and next action: %v", err)
	}
}

func TestDemoCommandHonoursEnvironmentFlag(t *testing.T) {
	spy := &demoRunnerSpy{result: demoResult()}
	if _, err := executeDemo(t, spy, nil, "", "--model", "ollama/x:1", "--environment", "k3d", "--output", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if spy.requests[0].Environment != "k3d" {
		t.Fatalf("environment = %q", spy.requests[0].Environment)
	}
}

func TestDemoAndTestUseSameRunnerContract(t *testing.T) {
	demoSpy := &demoRunnerSpy{result: demoResult()}
	if _, err := executeDemo(t, demoSpy, nil, "", "--model", "ollama/qwen3:8b", "--output", "/tmp/demo", "--environment", "k3d", "--timeout", "3m", "--ci"); err != nil {
		t.Fatalf("demo: %v", err)
	}
	testSpy := &demoRunnerSpy{result: demoResult()}
	testCmd := newTestCommand(testSpy.run)
	testCmd.SetOut(&bytes.Buffer{})
	testCmd.SetErr(&bytes.Buffer{})
	testCmd.SetArgs([]string{"--model", "ollama/qwen3:8b", "--output", "/tmp/demo", "--environment", "k3d", "--timeout", "3m", "--ci", "--suite", "kubernetes-demo@1"})
	if err := testCmd.Execute(); err != nil {
		t.Fatalf("test: %v", err)
	}
	if len(demoSpy.requests) != 1 || len(testSpy.requests) != 1 {
		t.Fatalf("requests = %d/%d", len(demoSpy.requests), len(testSpy.requests))
	}
	demo, test := demoSpy.requests[0], testSpy.requests[0]
	if demo.Model != test.Model || demo.Suite != test.Suite || demo.Environment != test.Environment ||
		demo.OutputDir != test.OutputDir || demo.Timeout != test.Timeout || demo.CI != test.CI ||
		demo.Agent != test.Agent || demo.Endpoint != test.Endpoint {
		t.Fatalf("demo request %+v diverges from test request %+v", demo, test)
	}
}

func TestDemoCommandRegisteredOnRoot(t *testing.T) {
	root := newRootCommand()
	found := false
	for _, child := range root.Commands() {
		if child.Name() == "demo" {
			found = true
		}
	}
	if !found {
		t.Fatal("demo command is not registered")
	}
}
