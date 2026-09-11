package harness

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/vitas/evidra-bench/pkg/environment"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

// provisionedIdentities couples an IdentityBundle with the provisioner and
// admin kubeconfig needed to revoke it, so the run path has a single
// teardown hook.
type provisionedIdentities struct {
	bundle          *environment.IdentityBundle
	provisioner     *environment.IdentityProvisioner
	adminKubeconfig string

	// convenience copies for the run path
	agent    *environment.Identity
	evidence *environment.Identity
}

func (p *provisionedIdentities) teardown(ctx context.Context) {
	if p == nil {
		return
	}
	// Revoke even on canceled runs; tokens must not outlive the run.
	tctx := context.WithoutCancel(ctx)
	if err := p.bundle.Teardown(tctx, p.provisioner, p.adminKubeconfig); err != nil {
		log.Printf("harness: identity teardown: %v", err)
	}
}

// provisionRunIdentities materializes the agent + evidence-reader service
// accounts for a scenario with an authority profile and applies the
// mandatory identity_auth_ready gate before returning them. The gate is not
// cosmetic: on kind, bearer authentication answers as system:anonymous for
// minutes after API start (tests/spikes/audit-provisioning/FINDINGS.md);
// an identity that has not passed the gate cannot support any windowed or
// attributed claim, and the agent must not start against a credential that
// is not yet honored.
func (h *Harness) provisionRunIdentities(ctx context.Context, req RunRequest, s *scenario.Scenario, adminKubeconfig string, recorder *runArtifactRecorder) (*provisionedIdentities, error) {
	runner := &environment.ExecRunner{}
	provisioner := environment.NewIdentityProvisioner(runner)
	bundle, err := provisioner.Provision(ctx, adminKubeconfig, s.AuthorityProfile)
	if err != nil {
		recorder.Event("identity", "failed", err.Error())
		return nil, &InfraError{Err: fmt.Errorf("harness.Run: materialize identities: %w", err)}
	}
	ready := identityReadyTimeout()
	if err := provisioner.WaitForAuthReady(ctx, bundle.Agent, ready); err != nil {
		recorder.Event("identity", "failed", err.Error())
		_ = bundle.Teardown(context.WithoutCancel(ctx), provisioner, adminKubeconfig)
		return nil, &InfraError{Err: err}
	}
	if err := provisioner.WaitForAuthReady(ctx, bundle.Evidence, ready); err != nil {
		recorder.Event("identity", "failed", err.Error())
		_ = bundle.Teardown(context.WithoutCancel(ctx), provisioner, adminKubeconfig)
		return nil, &InfraError{Err: err}
	}
	log.Printf("harness: run identities ready: agent=%s verifier=%s (markers=harness certificate identity)",
		bundle.Agent.User, bundle.Evidence.User)
	return &provisionedIdentities{
		bundle: bundle, provisioner: provisioner, adminKubeconfig: adminKubeconfig,
		agent: bundle.Agent, evidence: bundle.Evidence,
	}, nil
}

// identityReadyTimeout bounds the identity_auth_ready gate. The kind cold
// window measured ~190s on Linux CI; 10 minutes leaves headroom for slow
// runners while failing loudly instead of hanging forever.
func identityReadyTimeout() time.Duration {
	const fallback = 10 * time.Minute
	v := os.Getenv("EVIDRA_IDENTITY_READY_TIMEOUT")
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		log.Printf("harness: ignoring invalid EVIDRA_IDENTITY_READY_TIMEOUT %q", v)
		return fallback
	}
	return d
}
