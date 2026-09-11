package scenario

import (
	"fmt"
	"strings"
)

// AuthorityProfile is the explicit statement of what a run's identities are
// allowed to do and what must never be touched. It is the input to RBAC
// materialization (Phase 4) and to the authoritative verdict engine (Phase
// 8). In Phase 2 it is only parsed and validated; nothing consumes it for
// verdicts yet.
//
// Semantics (docs/adr/0001-process-safety-matching.md):
//   - Agent authority is exactly agent.rules + agent.cluster_scoped_rules.
//     Anything else the run identity touches is outside its grant.
//   - Protected entries are never auto-granted; an observed agent write to a
//     protected object is a violation regardless of what RBAC allowed.
//   - EvidenceReader describes the snapshot/verifier identity: read-only by
//     construction plus explicitly listed extra grants (e.g. pods/exec
//     create for service-reachable checks). The marker path is NOT part of
//     this profile: window markers ride the harness client-certificate
//     identity (spike finding: SA bearers authenticate late on kind).
//
// A scenario without an authority_profile is legal to load (mid-migration
// suites keep working) but can NEVER qualify: the gap
// "authority_profile_missing" marks the case as check-only forever.
type AuthorityProfile struct {
	Agent            AgentAuthority      `yaml:"agent"`
	Protected        []ProtectedResource `yaml:"protected,omitempty"`
	EvidenceReader   EvidenceReader      `yaml:"evidence_reader,omitempty"`
	AllowImpersonate bool                `yaml:"allow_impersonation,omitempty"`
}

// AgentAuthority declares the agent identity's namespace scope and rules.
type AgentAuthority struct {
	Namespaces []string     `yaml:"namespaces,omitempty"`
	Rules      []PolicyRule `yaml:"rules,omitempty"`
	// ClusterScopedRules require a ClusterRole; empty by default and
	// always explicit.
	ClusterScopedRules []PolicyRule `yaml:"cluster_scoped_rules,omitempty"`
	// OnDenied selects policy treatment of an authenticated 403 on a
	// mutating verb by the agent identity: "unsafe" (default) or
	// "warning".
	OnDenied string `yaml:"on_denied,omitempty"`
}

// PolicyRule mirrors the shape of a Kubernetes RBAC policy rule. Verbs are
// validated against the canonical RBAC verb set only (see
// canonicalRBACVerbs); non-canonical strings such as "describe" are a load
// error, not a silent widening.
type PolicyRule struct {
	APIGroups     []string `yaml:"apiGroups,omitempty"`
	Resources     []string `yaml:"resources,omitempty"`
	ResourceNames []string `yaml:"resource_names,omitempty"`
	// Subresources names channels of the listed resources ("exec",
	// "log", "scale"...); equivalently "pods/exec" inside Resources.
	// RBAC materializes the base/sub pairs; the verdict matcher treats an
	// unset Subresources as covering only base-object requests, so a
	// deployment write grant never authorizes pods/exec.
	Subresources []string `yaml:"subresources,omitempty"`
	// Namespaces restricts THIS rule to a subset of agent.namespaces
	// (loader-enforced). Empty = the profile-wide agent.namespaces,
	// preserving the historical cross-product for existing scenarios.
	Namespaces []string `yaml:"namespaces,omitempty"`
	Verbs      []string `yaml:"verbs"`
}

// ProtectedResource names a single object that agent writes to always
// constitute a violation.
type ProtectedResource struct {
	APIGroup  string `yaml:"apiGroup,omitempty"`
	Resource  string `yaml:"resource"`
	Namespace string `yaml:"namespace,omitempty"`
	Name      string `yaml:"name"`
}

// EvidenceReader declares the snapshot/verifier identity's reach. Resources
// are read-only by construction; Extra carries explicit non-read grants the
// verifiers need (canonical verb syntax, e.g. "pods/exec:create").
type EvidenceReader struct {
	Namespaces []string `yaml:"namespaces,omitempty"`
	Resources  []string `yaml:"resources,omitempty"`
	Extra      []string `yaml:"extra,omitempty"`
}

const (
	// OnDeniedUnsafe and OnDeniedWarning are the only legal
	// agent.on_denied values.
	OnDeniedUnsafe  = "unsafe"
	OnDeniedWarning = "warning"

	// GapAuthorityProfileMissing is recorded on results for scenarios
	// without a profile; it makes authoritative grading impossible.
	GapAuthorityProfileMissing = "authority_profile_missing"
)

// canonicalRBACVerbs is the set of verbs Kubernetes RBAC understands.
// "describe" is deliberately NOT here: kubectl describe issues
// get (+list/watch) requests; it is not an RBAC verb and must not leak into
// a verb list as a pseudo-grant.
var canonicalRBACVerbs = map[string]bool{
	"*":                true, // explicit wildcard
	"get":              true,
	"list":             true,
	"watch":            true,
	"create":           true,
	"update":           true,
	"patch":            true,
	"delete":           true,
	"deletecollection": true,
	"bind":             true,
	"escalate":         true,
	"impersonate":      true,
	"use":              true,
}

// IsCanonicalRBACVerb reports whether verb is a real Kubernetes RBAC verb
// (or the explicit wildcard "*"). Case-sensitive, like the API.
func IsCanonicalRBACVerb(verb string) bool {
	return canonicalRBACVerbs[verb]
}

// effectiveOnDenied returns the declared treatment or the unsafe default.
func (a AgentAuthority) effectiveOnDenied() string {
	if a.OnDenied == "" {
		return OnDeniedUnsafe
	}
	return a.OnDenied
}

// validateAuthorityProfile enforces schema correctness. A missing profile is
// not a load error (gap-pinned instead); a malformed one is.
func validateAuthorityProfile(s *Scenario) error {
	p := s.AuthorityProfile
	if p == nil {
		return nil
	}
	switch p.Agent.OnDenied {
	case "", OnDeniedUnsafe, OnDeniedWarning:
	default:
		return fmt.Errorf("scenario %s: authority_profile.agent.on_denied %q invalid (must be %q or %q)",
			s.ID, p.Agent.OnDenied, OnDeniedUnsafe, OnDeniedWarning)
	}
	ruleSets := []struct {
		name  string
		rules []PolicyRule
	}{
		{name: "authority_profile.agent.rules", rules: p.Agent.Rules},
		{name: "authority_profile.agent.cluster_scoped_rules", rules: p.Agent.ClusterScopedRules},
	}
	for _, set := range ruleSets {
		cluster := strings.HasSuffix(set.name, "cluster_scoped_rules")
		for i, r := range set.rules {
			if err := validatePolicyRule(s.ID, fmt.Sprintf("%s[%d]", set.name, i), r); err != nil {
				return err
			}
			if cluster && len(r.Namespaces) > 0 {
				return fmt.Errorf("scenario %s: %s[%d]: cluster_scoped rules cannot name namespaces", s.ID, set.name, i)
			}
			if !cluster {
				for _, ns := range r.Namespaces {
					if !containsStr(p.Agent.Namespaces, ns) {
						return fmt.Errorf("scenario %s: %s[%d]: namespace %q outside agent.namespaces %v", s.ID, set.name, i, ns, p.Agent.Namespaces)
					}
				}
			}
		}
	}
	for i, pr := range p.Protected {
		if pr.Resource == "" || pr.Name == "" {
			return fmt.Errorf("scenario %s: authority_profile.protected[%d] must set resource and name", s.ID, i)
		}
	}
	for i, e := range p.EvidenceReader.Extra {
		if err := validateEvidenceReaderExtra(s.ID, i, e); err != nil {
			return err
		}
	}
	if p.AllowImpersonate {
		if err := validateImpersonateRules(s.ID, p); err != nil {
			return err
		}
	}
	return nil
}

func validatePolicyRule(scenarioID, path string, r PolicyRule) error {
	if len(r.Verbs) == 0 {
		return fmt.Errorf("scenario %s: %s: verbs is required", scenarioID, path)
	}
	for _, v := range r.Verbs {
		if !IsCanonicalRBACVerb(v) {
			return fmt.Errorf("scenario %s: %s: verb %q is not a canonical Kubernetes RBAC verb (verbs get/list/watch/create/update/patch/delete/deletecollection/bind/escalate/impersonate/use or \"*\"; note: kubectl describe performs get/list — \"describe\" is not an RBAC verb)",
				scenarioID, path, v)
		}
	}
	for _, res := range r.Resources {
		if res == "" {
			return fmt.Errorf("scenario %s: %s: empty resource entry", scenarioID, path)
		}
		base, sub, hasSub := strings.Cut(res, "/")
		if hasSub && (base == "" || sub == "" || strings.Contains(sub, "/")) {
			return fmt.Errorf("scenario %s: %s: resource %q must be 'name' or 'name/subresource'", scenarioID, path, res)
		}
	}
	for _, sub := range r.Subresources {
		if sub == "" || strings.Contains(sub, "/") {
			return fmt.Errorf("scenario %s: %s: subresource %q must be a bare name (e.g. exec, log)", scenarioID, path, sub)
		}
	}
	return nil
}

func containsStr(list []string, x string) bool {
	for _, v := range list {
		if v == x {
			return true
		}
	}
	return false
}

// validateEvidenceReaderExtra accepts "resource" (read implied) or
// "resource:verb" with a canonical verb.
func validateEvidenceReaderExtra(scenarioID string, i int, entry string) error {
	resource, verb, hasVerb := strings.Cut(entry, ":")
	if strings.TrimSpace(resource) == "" {
		return fmt.Errorf("scenario %s: authority_profile.evidence_reader.extra[%d]: empty resource", scenarioID, i)
	}
	if !hasVerb {
		return nil
	}
	if !IsCanonicalRBACVerb(verb) {
		return fmt.Errorf("scenario %s: authority_profile.evidence_reader.extra[%d]: %q is not a canonical RBAC verb", scenarioID, i, verb)
	}
	return nil
}

// validateImpersonateRules requires impersonate authority to be stated as
// rules (targeted), never implied by the boolean alone.
func validateImpersonateRules(scenarioID string, p *AuthorityProfile) error {
	for _, r := range append(append([]PolicyRule{}, p.Agent.Rules...), p.Agent.ClusterScopedRules...) {
		for _, v := range r.Verbs {
			if v == "impersonate" || v == "*" {
				return nil
			}
		}
	}
	return fmt.Errorf("scenario %s: authority_profile.allow_impersonation is true but no rule grants the impersonate verb", scenarioID)
}
