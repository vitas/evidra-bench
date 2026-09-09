package main

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vitas/evidra-bench/pkg/agent"
	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/evaluation"
	"github.com/vitas/evidra-bench/pkg/modelconfig"
	"github.com/vitas/evidra-bench/pkg/suite"
)

type eventRecorder struct {
	events []string
}

func (r *eventRecorder) add(names ...string) {
	r.events = append(r.events, names...)
}

type orderingProvisioner struct {
	rec  *eventRecorder
	fail error
}

func (p orderingProvisioner) Acquire(ctx context.Context, _ evaluation.Plan) (evaluation.Lease, error) {
	if p.fail != nil {
		return nil, p.fail
	}
	p.rec.add("acquire")
	return orderingLease{rec: p.rec}, nil
}

type orderingLease struct {
	rec *eventRecorder
}

func (l orderingLease) Release(context.Context) error {
	l.rec.add("release")
	return nil
}

type orderingExecutor struct {
	rec *eventRecorder
}

func (e orderingExecutor) Execute(_ context.Context, _ evaluation.Plan, c evaluation.CasePlan, _ evaluation.Lease) (evaluation.CaseResult, error) {
	e.rec.add("execute")
	return evaluation.CaseResult{ScenarioID: c.ID, Verdict: evaluation.VerdictPass}, nil
}

func localModelTestRequest(outputDir string) testRequest {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	return testRequest{
		Model:       "ollama/qwen3:8b",
		Suite:       "kubernetes-demo@1",
		Environment: "kind",
		OutputDir:   outputDir,
		ProjectRoot: root,
		Timeout:     defaultTestCaseTimeout,
	}
}

func TestLocalModelPreparationCompletesBeforeClusterAcquisition(t *testing.T) {
	rec := &eventRecorder{}
	deps := testRuntimeDeps{
		LookupEnv:  func(string) string { return "" },
		Preflights: nil,
		PrepareOllama: func(_ context.Context, resolved modelconfig.Resolved, required []string) (modelconfig.PreparedModel, error) {
			if resolved.Provider != "ollama" || resolved.Model != "qwen3:8b" {
				t.Errorf("prepare received %+v", resolved)
			}
			if len(required) == 0 || required[0] != "tools" {
				t.Errorf("prepare required capabilities = %v, want suite-declared [tools]", required)
			}
			rec.add("discover", "show", "probe")
			return modelconfig.PreparedModel{
				Name:            resolved.Model,
				Digest:          "sha256:abc",
				ParameterSize:   "8.4B",
				Quantization:    "Q4_K_M",
				CapabilityCheck: modelconfig.CapabilityCheckBehavioralProbe,
				ProbeUsage:      &agent.Usage{PromptTokens: 7, CompletionTokens: 3},
			}, nil
		},
		NewProvisioner: func(config.Config, *suite.Loaded) evaluation.Provisioner {
			return orderingProvisioner{rec: rec}
		},
		NewExecutor: func(config.Config, *suite.Loaded, agent.Provider) evaluation.CaseExecutor {
			return orderingExecutor{rec: rec}
		},
	}

	result, err := runOneCommandEvaluationWith(context.Background(), localModelTestRequest(t.TempDir()), deps)
	if err != nil {
		t.Fatalf("runOneCommandEvaluationWith() error = %v", err)
	}
	if len(rec.events) == 0 || rec.events[0] != "discover" {
		t.Fatalf("events = %v, want discovery first", rec.events)
	}
	acquireIndex := -1
	for i, event := range rec.events {
		if event == "acquire" {
			acquireIndex = i
			break
		}
	}
	if acquireIndex != 3 {
		t.Fatalf("events = %v, want discover,show,probe before acquire", rec.events)
	}
	if rec.events[len(rec.events)-1] != "release" {
		t.Fatalf("events = %v, want release last", rec.events)
	}
	if result.Target.ModelDigest != "sha256:abc" || result.Target.Quantization != "Q4_K_M" ||
		result.Target.ParameterSize != "8.4B" || result.Target.CapabilityCheck != modelconfig.CapabilityCheckBehavioralProbe {
		t.Fatalf("result target = %+v, want prepared local model identity", result.Target)
	}
	if len(result.Preflight) != 1 || result.Preflight[0].Kind != "model_capability" ||
		result.Preflight[0].Method != modelconfig.CapabilityCheckBehavioralProbe ||
		!result.Preflight[0].Usage.Known || result.Preflight[0].Usage.PromptTokens != 7 ||
		result.Preflight[0].Usage.CompletionTokens != 3 {
		t.Fatalf("preflight evidence = %+v", result.Preflight)
	}
}

func TestLocalModelPreflightFailuresNeverAcquireCluster(t *testing.T) {
	for _, tt := range []struct {
		name string
		fail error
	}{
		{name: "runtime unreachable", fail: errors.New("ollama runtime unreachable: connection refused")},
		{name: "model missing", fail: errors.New(`ollama model "qwen3:8b" is not installed`)},
		{name: "capability absent", fail: errors.New("ollama model \"qwen3:8b\" does not support tool calling")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := &eventRecorder{}
			deps := testRuntimeDeps{
				LookupEnv: func(string) string { return "" },
				PrepareOllama: func(context.Context, modelconfig.Resolved, []string) (modelconfig.PreparedModel, error) {
					rec.add("discover")
					return modelconfig.PreparedModel{}, tt.fail
				},
				NewProvisioner: func(config.Config, *suite.Loaded) evaluation.Provisioner {
					return orderingProvisioner{rec: rec}
				},
				NewExecutor: func(config.Config, *suite.Loaded, agent.Provider) evaluation.CaseExecutor {
					return orderingExecutor{rec: rec}
				},
			}
			if _, err := runOneCommandEvaluationWith(context.Background(), localModelTestRequest(t.TempDir()), deps); err == nil || !strings.Contains(err.Error(), tt.fail.Error()) {
				t.Fatalf("error = %v, want %v", err, tt.fail)
			}
			for _, event := range rec.events {
				if event == "acquire" || event == "execute" {
					t.Fatalf("failed preflight must never reach the cluster: events = %v", rec.events)
				}
			}
		})
	}
}

func TestPrepareSkipsOllamaPreparationForOtherProviders(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	preparedPrepareCalls := 0
	deps := testRuntimeDeps{
		PrepareOllama: func(context.Context, modelconfig.Resolved, []string) (modelconfig.PreparedModel, error) {
			preparedPrepareCalls++
			return modelconfig.PreparedModel{}, errors.New("must not run")
		},
	}
	prepared, err := prepareTestEvaluation(context.Background(), testRequest{
		Model:       "my-model",
		Endpoint:    "http://127.0.0.1:9/v1",
		Suite:       "kubernetes-demo@1",
		Environment: "kind",
		OutputDir:   t.TempDir(),
		ProjectRoot: root,
		Timeout:     defaultTestCaseTimeout,
	}, func(string) string { return "" }, deps)
	if err != nil {
		t.Fatalf("prepareTestEvaluation() error = %v", err)
	}
	if preparedPrepareCalls != 0 {
		t.Fatal("custom OpenAI-compatible endpoints must not run Ollama discovery")
	}
	if prepared.Plan.Target.CapabilityCheck != "" || prepared.Plan.Target.ModelDigest != "" {
		t.Fatalf("custom endpoint target gained local identity: %+v", prepared.Plan.Target)
	}
	encoded, err := json.Marshal(prepared.Plan)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "127.0.0.1:9") {
		t.Fatalf("plan leaked private endpoint: %s", encoded)
	}
}

func TestOllamaPreparationTimeoutHonoursContext(t *testing.T) {
	rec := &eventRecorder{}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	deps := testRuntimeDeps{
		LookupEnv: func(string) string { return "" },
		PrepareOllama: func(ctx context.Context, _ modelconfig.Resolved, _ []string) (modelconfig.PreparedModel, error) {
			<-ctx.Done()
			rec.add("discover")
			return modelconfig.PreparedModel{}, ctx.Err()
		},
		NewProvisioner: func(config.Config, *suite.Loaded) evaluation.Provisioner {
			return orderingProvisioner{rec: rec}
		},
		NewExecutor: func(config.Config, *suite.Loaded, agent.Provider) evaluation.CaseExecutor {
			return orderingExecutor{rec: rec}
		},
	}
	if _, err := runOneCommandEvaluationWith(ctx, localModelTestRequest(t.TempDir()), deps); err == nil {
		t.Fatal("expected context deadline error")
	}
	for _, event := range rec.events {
		if event == "acquire" {
			t.Fatal("timed-out discovery must not acquire")
		}
	}
}
