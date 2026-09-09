package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vitas/evidra-bench/pkg/evaluation"
)

func TestPrepareTestEvaluationBuildsCanonicalCredentialFreePlan(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareTestEvaluation(testRequest{
		Model:       "openai/gpt-test",
		Suite:       "kubernetes-demo@1",
		Environment: "kind",
		OutputDir:   t.TempDir(),
		ProjectRoot: root,
		Timeout:     defaultTestCaseTimeout,
	}, func(name string) string {
		if name == "OPENAI_API_KEY" {
			return "super-secret"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("prepareTestEvaluation() error = %v", err)
	}
	if prepared.Plan.Target.Kind != evaluation.TargetModel || prepared.Plan.Target.Provider != "openai" {
		t.Fatalf("target = %#v", prepared.Plan.Target)
	}
	if len(prepared.Plan.Suite.Cases) != 3 || prepared.Plan.Environment.Provider != "kind" {
		t.Fatalf("plan = %#v", prepared.Plan)
	}
	encoded, err := json.Marshal(prepared.Plan)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "super-secret") || strings.Contains(string(encoded), "api.openai.com") {
		t.Fatalf("plan leaked runtime provider configuration: %s", encoded)
	}
}

func TestPrepareTestEvaluationRejectsUnsupportedEnvironmentBeforeProvisioning(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	_, err := prepareTestEvaluation(testRequest{
		Agent:       "./agent",
		Suite:       "kubernetes-demo@1",
		Environment: "minikube",
		OutputDir:   t.TempDir(),
		ProjectRoot: root,
		Timeout:     defaultTestCaseTimeout,
	}, func(string) string { return "" })
	if err == nil || !strings.Contains(err.Error(), "kind or k3d") {
		t.Fatalf("error = %v", err)
	}
}
