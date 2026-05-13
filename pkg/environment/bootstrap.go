package environment

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// StepType identifies the kind of bootstrap action.
type StepType string

const (
	StepKubectlApply  StepType = "kubectl-apply"
	StepKubectlCreate StepType = "kubectl-create"
	StepKubectl       StepType = "kubectl"
	StepHelmInstall   StepType = "helm-install"
	StepShell         StepType = "shell"
	StepSleep         StepType = "sleep"
)

// BootstrapStep describes a single bootstrap action.
type BootstrapStep struct {
	Name      string
	Type      StepType
	Path      string
	Feature   string
	Release   string
	Namespace string
	Duration  string
	Args      []string
}

// CommandArgs returns the shell command arguments for this step.
func (s *BootstrapStep) CommandArgs(kubeconfigPath string) []string {
	switch s.Type {
	case StepKubectlApply:
		args := []string{"kubectl", "--kubeconfig", kubeconfigPath}
		if s.Namespace != "" {
			args = append(args, "-n", s.Namespace)
		}
		args = append(args, "apply", "-f", s.Path)
		return args
	case StepKubectlCreate:
		args := []string{"kubectl", "--kubeconfig", kubeconfigPath, "create"}
		if s.Namespace != "" {
			args = append(args, "-n", s.Namespace)
		}
		return append(args, s.Args...)
	case StepKubectl:
		args := []string{"kubectl", "--kubeconfig", kubeconfigPath}
		return append(args, s.Args...)
	case StepHelmInstall:
		args := []string{"helm", "--kubeconfig", kubeconfigPath, "upgrade", "--install", s.Release, s.Path}
		if s.Namespace != "" {
			args = append(args, "-n", s.Namespace)
		}
		return append(args, s.Args...)
	case StepShell:
		// Shell steps get kubeconfigPath as first arg after the script path.
		args := []string{"bash", s.Path, kubeconfigPath}
		return append(args, s.Args...)
	case StepSleep:
		return nil
	default:
		return nil
	}
}

// BootstrapPlan is an ordered list of bootstrap steps.
type BootstrapPlan struct {
	Steps []BootstrapStep
}

// Requires returns true if the plan includes a step with the given feature.
func (p *BootstrapPlan) Requires(feature string) bool {
	for _, s := range p.Steps {
		if s.Feature == feature {
			return true
		}
	}
	return false
}

// DefaultBootstrapPlan returns the common bootstrap plan shared by all scenarios.
func DefaultBootstrapPlan() *BootstrapPlan {
	return &BootstrapPlan{
		Steps: []BootstrapStep{
			{
				Name:    "apply-bench-namespace",
				Type:    StepKubectlApply,
				Path:    "manifests/core/bench-namespace.yaml",
				Feature: "namespace",
			},
		},
	}
}

// Bootstrapper executes a bootstrap plan against a cluster.
type Bootstrapper struct {
	Runner CommandRunner
}

// NewBootstrapper returns a Bootstrapper with the given runner.
func NewBootstrapper(runner CommandRunner) *Bootstrapper {
	return &Bootstrapper{Runner: runner}
}

// Execute runs all steps in the plan sequentially.
func (b *Bootstrapper) Execute(ctx context.Context, plan *BootstrapPlan, kubeconfigPath string) error {
	for _, step := range plan.Steps {
		if step.Type == StepSleep {
			duration, err := time.ParseDuration(step.Duration)
			if err != nil {
				return fmt.Errorf("environment.Bootstrapper.Execute: step %s: parse duration %q: %w", step.Name, step.Duration, err)
			}
			timer := time.NewTimer(duration)
			select {
			case <-ctx.Done():
				timer.Stop()
				return fmt.Errorf("environment.Bootstrapper.Execute: step %s: %w", step.Name, ctx.Err())
			case <-timer.C:
			}
			continue
		}
		args := step.CommandArgs(kubeconfigPath)
		if len(args) == 0 {
			return fmt.Errorf("environment.Bootstrapper.Execute: no command for step %s", step.Name)
		}
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		out, err := b.Runner.Run(ctx, cmd)
		if err != nil {
			message := fmt.Sprintf("environment.Bootstrapper.Execute: step %s: %v", step.Name, err)
			if trimmed := strings.TrimSpace(string(out)); trimmed != "" {
				message = fmt.Sprintf("%s: %s", message, trimmed)
			}
			return fmt.Errorf("%s", message)
		}
	}
	return nil
}
