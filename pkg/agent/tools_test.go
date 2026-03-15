package agent

import (
	"strings"
	"testing"

	"samebits.com/evidra/pkg/execcontract"
)

func TestBuildPrescribeCommandArgs_UsesActorMetadata(t *testing.T) {
	t.Parallel()

	args, err := buildPrescribeCommandArgs("/tmp/evidence", "/tmp/artifact.yaml", execcontract.PrescribeInput{
		Tool:         "kubectl",
		Operation:    "apply",
		RawArtifact:  "apiVersion: v1\nkind: ConfigMap\n",
		TraceID:      "trace-123",
		SpanID:       "span-123",
		ParentSpanID: "span-root",
		ScopeDimensions: map[string]string{
			"cluster":   "kind-evidra",
			"namespace": "bench",
		},
		Actor: execcontract.Actor{
			Type:         "agent",
			ID:           "bench-agent",
			Origin:       "mcp-stdio",
			InstanceID:   "session-123",
			Version:      "claude-sonnet-4.5",
			SkillVersion: "1.0.1",
		},
	})
	if err != nil {
		t.Fatalf("buildPrescribeCommandArgs: %v", err)
	}

	got := strings.Join(args, " ")
	for _, want := range []string{
		"prescribe",
		"--actor bench-agent",
		"--actor-type agent",
		"--actor-origin mcp-stdio",
		"--actor-instance-id session-123",
		"--actor-version claude-sonnet-4.5",
		"--actor-skill-version 1.0.1",
		"--trace-id trace-123",
		"--span-id span-123",
		"--parent-span-id span-root",
		"--scope-dimensions {\"cluster\":\"kind-evidra\",\"namespace\":\"bench\"}",
		"--tool kubectl",
		"--operation apply",
		"-f /tmp/artifact.yaml",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("args %q missing %q", got, want)
		}
	}
}

func TestBuildReportCommandArgs_DeclinedOmitsExitCode(t *testing.T) {
	t.Parallel()

	args, err := buildReportCommandArgs("/tmp/evidence", execcontract.ReportInput{
		PrescriptionID: "presc-1",
		Verdict:        execcontract.VerdictDeclined,
		DecisionContext: &execcontract.DecisionContext{
			Trigger: "risk_threshold_exceeded",
			Reason:  "blast radius too large",
		},
		Actor: execcontract.Actor{
			Type:         "agent",
			ID:           "bench-agent",
			Origin:       "mcp-stdio",
			SkillVersion: "1.0.1",
		},
		SpanID:       "span-123",
		ParentSpanID: "span-root",
	})
	if err != nil {
		t.Fatalf("buildReportCommandArgs: %v", err)
	}

	got := strings.Join(args, " ")
	if strings.Contains(got, "--exit-code") {
		t.Fatalf("declined report args must not include --exit-code: %q", got)
	}
	for _, want := range []string{
		"--decline-trigger risk_threshold_exceeded",
		"--decline-reason blast radius too large",
		"--actor bench-agent",
		"--actor-type agent",
		"--actor-origin mcp-stdio",
		"--actor-skill-version 1.0.1",
		"--span-id span-123",
		"--parent-span-id span-root",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("args %q missing %q", got, want)
		}
	}
}
