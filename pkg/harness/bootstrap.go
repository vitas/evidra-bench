package harness

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/environment"
	"samebits.com/evidra-infra-bench/pkg/scenario"
)

func (h *Harness) applyBreak(ctx context.Context, kubeconfigPath string, s *scenario.Scenario, extraEnv ...string) error {
	runner := bootstrapperRunner(h.deps.Bootstrapper)
	args, err := breakCommandArgs(kubeconfigPath, s)
	if err != nil {
		return err
	}
	cmd := makeCmd(args)
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	if _, err := runner.Run(ctx, cmd); err != nil {
		if s.Break.AllowFailure {
			log.Printf("[harness] break command failed as expected for scenario %s: %v", s.ID, err)
			return nil
		}
		return fmt.Errorf("apply break fixture: %w", err)
	}
	return nil
}

func buildBootstrapPlan(s *scenario.Scenario, scenariosDir string) *environment.BootstrapPlan {
	plan := environment.DefaultBootstrapPlan()
	rootDir := filepath.Dir(scenariosDir)
	for i := range plan.Steps {
		if plan.Steps[i].Path != "" && rootDir != "" && !filepath.IsAbs(plan.Steps[i].Path) {
			plan.Steps[i].Path = filepath.Join(rootDir, plan.Steps[i].Path)
		}
	}
	plan.Steps = append(plan.Steps, buildStepPlan(s.Bootstrap).Steps...)
	return plan
}

func buildStepPlan(steps []scenario.BootstrapStep) *environment.BootstrapPlan {
	plan := &environment.BootstrapPlan{}
	for _, step := range steps {
		plan.Steps = append(plan.Steps, environment.BootstrapStep{
			Name:      step.Name,
			Type:      environment.StepType(step.Type),
			Path:      step.Path,
			Release:   step.Release,
			Namespace: step.Namespace,
			Duration:  step.Duration,
			Args:      append([]string(nil), step.Args...),
		})
	}
	return plan
}

// waitForRollouts gives deployments time to converge after the agent finishes.
func waitForRollouts(ctx context.Context, kubeconfigPath string, s *scenario.Scenario) {
	for _, check := range s.Checks {
		if check.Type != "deployment-ready" {
			continue
		}
		ns := check.Namespace
		if ns == "" {
			ns = config.DefaultNamespace
		}
		waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		cmd := exec.CommandContext(waitCtx, "kubectl",
			"--kubeconfig", kubeconfigPath,
			"rollout", "status",
			fmt.Sprintf("deployment/%s", check.Name),
			"-n", ns,
			"--timeout=30s",
		)
		out, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			log.Printf("[harness] rollout wait for %s/%s: %v: %s", ns, check.Name, err, strings.TrimSpace(string(out)))
		}
	}
}

// bootstrapperRunner returns the runner from the bootstrapper, or a default ExecRunner.
func bootstrapperRunner(b *environment.Bootstrapper) environment.CommandRunner {
	if b != nil && b.Runner != nil {
		return b.Runner
	}
	return &environment.ExecRunner{}
}

func breakCommandArgs(kubeconfigPath string, s *scenario.Scenario) ([]string, error) {
	switch s.Break.Type {
	case "", "apply", "kubectl-apply":
		if s.Break.Path == "" {
			return nil, fmt.Errorf("break fixture path is required for %q", s.Break.Type)
		}
		return []string{"kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", s.Break.Path}, nil
	case "kubectl":
		args := []string{"kubectl", "--kubeconfig", kubeconfigPath}
		return append(args, s.Break.Args...), nil
	case "shell":
		args := []string{"bash", s.Break.Path, kubeconfigPath}
		return append(args, s.Break.Args...), nil
	case "helm-upgrade":
		if s.Break.Name == "" {
			return nil, fmt.Errorf("break release name is required for helm-upgrade")
		}
		if s.Break.Chart == "" {
			return nil, fmt.Errorf("break chart is required for helm-upgrade")
		}
		args := []string{"helm", "--kubeconfig", kubeconfigPath, "upgrade", s.Break.Name, s.Break.Chart}
		if s.Break.Namespace != "" {
			args = append(args, "-n", s.Break.Namespace)
		}
		if s.Break.Path != "" {
			args = append(args, "-f", s.Break.Path)
		}
		return append(args, s.Break.Args...), nil
	default:
		return nil, fmt.Errorf("unsupported break type: %s", s.Break.Type)
	}
}
