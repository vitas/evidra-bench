package environment

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"sort"

	"github.com/vitas/evidra-bench/pkg/authority"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

// Per-run identities materialized from a scenario authority profile
// (docs/adr/0001, plan Phase 4). Three distinct identities, never shared:
//
//	agent          — service account whose RBAC is the authority profile
//	                 verbatim; the adapter runs with this kubeconfig.
//	evidence reader— service account used by verifiers/snapshot readers.
//	marker         — the harness client-certificate (admin) identity itself;
//	                 spike-proven immune to the kind bearer cold window, so
//	                 window markers are GETs of nonce ConfigMaps made with
//	                 this identity (EmitMarker).
//
// Every SA identity must pass WaitForAuthReady (identity_auth_ready gate)
// before any windowed claim about its actions is trusted.

const (
	// IdentityNamespace hosts the service accounts and marker ConfigMaps.
	IdentityNamespace = "evidra-system"
	// AgentServiceAccount is the run-scoped agent identity name.
	AgentServiceAccount = "evidra-agent"
	// EvidenceServiceAccount is the run-scoped evidence reader identity.
	EvidenceServiceAccount = "evidra-evidence"
	// AgentUserName / EvidenceUserName are the principal names as they
	// appear in audit events.
	AgentUserName     = "system:serviceaccount:" + IdentityNamespace + ":" + AgentServiceAccount
	EvidenceUserName  = "system:serviceaccount:" + IdentityNamespace + ":" + EvidenceServiceAccount
	tokenDurationReqs = "2h"
	probeNamePrefix   = "evidra-auth-probe-"
	markerNamePrefix  = "evidra-marker-"
)

// Identity is one materialized run identity with a usable kubeconfig.
type Identity struct {
	User           string // principal name as seen in audit
	KubeconfigPath string
}

// IdentityBundle owns the on-cluster objects and local files created for one
// run. Teardown removes everything; the bundle is not safe for reuse.
type IdentityBundle struct {
	dir      string
	files    []string // applied manifest files (delete -f on teardown)
	Agent    *Identity
	Evidence *Identity
}

// IdentityProvisioner materializes identities with kubectl through the
// provided CommandRunner (the same abstraction providers use).
type IdentityProvisioner struct {
	Runner CommandRunner
}

// NewIdentityProvisioner builds a provisioner.
func NewIdentityProvisioner(runner CommandRunner) *IdentityProvisioner {
	return &IdentityProvisioner{Runner: runner}
}

func (p *IdentityProvisioner) kctl(ctx context.Context, kubeconfig string, args ...string) ([]byte, error) {
	full := append([]string{"--kubeconfig", kubeconfig}, args...)
	//nolint:gosec // kubectl with a fixed binary name; args are constructed internally.
	cmd := exec.CommandContext(ctx, "kubectl", full...)
	return p.Runner.Run(ctx, cmd)
}

// Provision creates the identities dictated by the profile. A nil profile
// means the scenario carries no authority contract: no identities are
// materialized and the caller keeps using the admin kubeconfig (the run is
// permanent evidence gaps for other reasons — authority_profile_missing).
func (p *IdentityProvisioner) Provision(ctx context.Context, adminKubeconfig string, profile *scenario.AuthorityProfile) (*IdentityBundle, error) {
	if profile == nil {
		return nil, nil
	}
	dir, err := os.MkdirTemp("", "evidra-identity-")
	if err != nil {
		return nil, fmt.Errorf("identity: temp dir: %w", err)
	}
	b := &IdentityBundle{dir: dir}
	cleanup := func() { _ = os.RemoveAll(dir) }

	if err := ValidateProfileRules(profile); err != nil {
		cleanup()
		return nil, err
	}
	if err := p.applyAll(ctx, adminKubeconfig, b, identityManifests(profile)); err != nil {
		cleanup()
		return nil, err
	}

	cluster, err := p.clusterInfo(ctx, adminKubeconfig)
	if err != nil {
		cleanup()
		return nil, err
	}

	agentToken, err := p.mintToken(ctx, adminKubeconfig, b, AgentServiceAccount)
	if err != nil {
		cleanup()
		return nil, err
	}
	agentKC := filepath.Join(dir, "agent.kubeconfig.json")
	if err := writeKubeconfig(agentKC, cluster, agentToken); err != nil {
		cleanup()
		return nil, err
	}
	b.Agent = &Identity{User: AgentUserName, KubeconfigPath: agentKC}

	evToken, err := p.mintToken(ctx, adminKubeconfig, b, EvidenceServiceAccount)
	if err != nil {
		cleanup()
		return nil, err
	}
	evKC := filepath.Join(dir, "evidence.kubeconfig.json")
	if err := writeKubeconfig(evKC, cluster, evToken); err != nil {
		cleanup()
		return nil, err
	}
	b.Evidence = &Identity{User: EvidenceUserName, KubeconfigPath: evKC}

	return b, nil
}

// Teardown revokes the run identities: every applied manifest object is
// deleted and local kubeconfig files (which carry live tokens) are removed.
// Deleting a cluster destroys these objects anyway; this path matters for
// reused/pre-provided clusters (RunRequest.KubeconfigPath).
func (b *IdentityBundle) Teardown(ctx context.Context, p *IdentityProvisioner, adminKubeconfig string) error {
	if b == nil {
		return nil
	}
	var firstErr error
	for i := len(b.files) - 1; i >= 0; i-- {
		if _, err := p.kctl(ctx, adminKubeconfig, "delete", "--ignore-not-found", "--wait=false", "-f", b.files[i]); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("identity teardown: delete %s: %w", filepath.Base(b.files[i]), err)
		}
	}
	_, _ = p.kctl(ctx, adminKubeconfig, "delete", "namespace", IdentityNamespace, "--ignore-not-found", "--wait=false")
	if b.dir != "" {
		_ = os.RemoveAll(b.dir)
	}
	return firstErr
}

func (p *IdentityProvisioner) applyAll(ctx context.Context, adminKubeconfig string, b *IdentityBundle, objs []map[string]any) error {
	for i, obj := range objs {
		path := filepath.Join(b.dir, fmt.Sprintf("manifest-%02d.json", i))
		data, err := json.Marshal(obj)
		if err != nil {
			return fmt.Errorf("identity: marshal %v: %w", obj["kind"], err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return fmt.Errorf("identity: write manifest: %w", err)
		}
		if out, err := p.kctl(ctx, adminKubeconfig, "apply", "-f", path); err != nil {
			return fmt.Errorf("identity: apply %s (%s): %w: %s", obj["kind"], obj["metadata"], err, out)
		}
		b.files = append(b.files, path)
	}
	return nil
}

// --- manifest materialization -------------------------------------------------

// identityManifests renders every object dictated by the profile, in apply
// order: namespace, service accounts, agent roles/binding per namespace,
// cluster role/binding when requested, evidence-reader roles/bindings.
func identityManifests(profile *scenario.AuthorityProfile) []map[string]any {
	objs := []map[string]any{
		obj("v1", "Namespace", IdentityNamespace, ""),
		obj("v1", "ServiceAccount", AgentServiceAccount, IdentityNamespace),
		obj("v1", "ServiceAccount", EvidenceServiceAccount, IdentityNamespace),
	}

	// ONE compiler for every consumer (ADR 0001 review): RBAC below, the
	// verdict matcher, and the snapshot scope all derive from the same
	// compiled plan — per-rule namespace scopes stay per-rule, so a write
	// grant confined to bench-staging materializes ONLY there. The
	// escalation-verb filter is defense in depth on top of the loader
	// validator (impersonate/escalate/bind never reach the API).
	plan, err := authority.Compile(profile, "")
	if err != nil {
		// The scenario loader validated the profile; a compile error here
		// can only mean an out-of-scope rule namespace. Fail closed.
		panic("identity: authority compile: " + err.Error())
	}
	roles := plan.RolesByNamespace()
	names := make([]string, 0, len(roles))
	for ns := range roles {
		names = append(names, ns)
	}
	sort.Strings(names)
	for _, ns := range names {
		roleName := "evidra-agent-role-" + dnsLabel(ns)
		objs = append(objs, roleObject(roleName, ns, rbacMaps(roles[ns])))
		objs = append(objs, roleBinding(roleName, ns, AgentServiceAccount))
	}
	if crs := plan.ClusterRoleRules(); len(crs) > 0 {
		objs = append(objs, clusterRole("evidra-agent-cluster-role", rbacMaps(crs)))
		objs = append(objs, clusterRoleBinding("evidra-agent-cluster-role", AgentServiceAccount))
	}

	evNamespaces := evidenceNamespaces(profile)
	evRules := []map[string]any{{
		"apiGroups": []string{"", "apps"},
		"resources": strList(evResources(profile)),
		"verbs":     []string{"get", "list", "watch"},
	}}
	for _, extra := range profile.EvidenceReader.Extra {
		res, verb, _ := strings.Cut(extra, ":")
		group := ""
		base := res
		if idx := strings.Index(res, "/"); idx >= 0 {
			base = res[:idx]
		}
		switch base {
		case "deployments", "statefulsets", "daemonsets", "replicasets":
			group = "apps"
		}
		evRules = append(evRules, map[string]any{
			"apiGroups": []string{group},
			"resources": []string{res},
			"verbs":     []string{verb},
		})
	}
	for _, ns := range evNamespaces {
		objs = append(objs, roleObject("evidra-evidence-role", ns, evRules))
		objs = append(objs, roleBinding("evidra-evidence-role", ns, EvidenceServiceAccount))
	}
	return objs
}

// ValidateProfileRules surfaces materialization-time refusals: an agent rule
// that would grant a write on a protected object can never be materialized
// (reads are allowed — the agent must be able to see what it must not break).
func ValidateProfileRules(profile *scenario.AuthorityProfile) error {
	if profile == nil {
		return nil
	}
	// Per-rule namespace scopes are honored: a staging-confined write
	// grant does NOT collide with a protected object in bench — the
	// historical cross-product here (and in RBAC) punished exactly the
	// narrowing this compiler exists to make expressible.
	scopeOf := func(rule scenario.PolicyRule) []string {
		if len(rule.Namespaces) > 0 {
			return rule.Namespaces
		}
		return profile.Agent.Namespaces
	}
	var out []string
	for _, rule := range profile.Agent.Rules {
		mutating := false
		for _, v := range rule.Verbs {
			if v == "*" || !readVerbs[v] {
				mutating = true
			}
		}
		if !mutating {
			continue
		}
		for _, prot := range profile.Protected {
			resMatch := strContains(rule.Resources, prot.Resource) || strContains(rule.Resources, "*")
			if !resMatch {
				continue
			}
			if !strContains(scopeOf(rule), prot.Namespace) {
				continue
			}
			if len(rule.ResourceNames) == 0 {
				out = append(out, fmt.Sprintf("%s: verbs %v on %s/%s granted by wildcard (protected: %s/%s)",
					prot.Namespace, rule.Verbs, prot.Namespace, prot.Resource, prot.Namespace, prot.Name))
			} else if strContains(rule.ResourceNames, prot.Name) {
				out = append(out, fmt.Sprintf("%s: verbs %v explicitly granted on protected %s/%s",
					prot.Namespace, rule.Verbs, prot.Namespace, prot.Name))
			}
		}
	}
	// cluster_scoped_rules can never reach a namespaced protected object
	// (RBAC semantics, not policy preference), so they are not checked
	// against the protected list.
	if len(out) > 0 {
		return fmt.Errorf("identity: authority profile grants agent writes on protected resources: %s", strings.Join(dedupe(out), "; "))
	}
	return nil
}

var readVerbs = map[string]bool{"get": true, "list": true, "watch": true}

// sanitizeRules drops privileged-escalation verbs regardless of what the
// profile said (the loader gate already requires allow_impersonation +
// explicit rule; this is defense in depth for materialization).
// rbacMaps converts compiled rules into unstructured manifests.
func rbacMaps(rules []authority.RBACRule) []map[string]any {
	out := make([]map[string]any, 0, len(rules))
	for _, r := range rules {
		m := map[string]any{
			"apiGroups": strList(r.APIGroups),
			"resources": strList(r.Resources),
			"verbs":     strList(r.Verbs),
		}
		if len(r.ResourceNames) > 0 {
			m["resourceNames"] = strList(r.ResourceNames)
		}
		out = append(out, m)
	}
	return out
}

// dnsLabel turns a namespace into a deterministic RBAC role-name suffix.
func dnsLabel(ns string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(ns) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteByte(byte(r))
		} else {
			b.WriteByte('-')
		}
	}
	out := b.String()
	if len(out) > 40 {
		out = out[:40]
	}
	return strings.Trim(out, "-")
}

func evResources(profile *scenario.AuthorityProfile) []string {
	res := append([]string{}, profile.EvidenceReader.Resources...)
	if len(res) == 0 {
		res = []string{"pods", "services", "endpoints", "events", "namespaces"}
		for _, p := range profile.Protected {
			res = append(res, p.Resource)
		}
	}
	return dedupe(res)
}

func evidenceNamespaces(profile *scenario.AuthorityProfile) []string {
	ns := append([]string{}, profile.EvidenceReader.Namespaces...)
	if len(profile.EvidenceReader.Namespaces) == 0 {
		ns = append(ns, profile.Agent.Namespaces...)
		for _, p := range profile.Protected {
			ns = append(ns, p.Namespace)
		}
	}
	ns = dedupe(ns)
	if len(ns) == 0 {
		ns = []string{"default"}
	}
	return ns
}

func roleObject(name, ns string, rules []map[string]any) map[string]any {
	r := obj("rbac.authorization.k8s.io/v1", "Role", name, ns)
	r["rules"] = cleanRules(rules)
	return r
}

func clusterRole(name string, rules []map[string]any) map[string]any {
	r := obj("rbac.authorization.k8s.io/v1", "ClusterRole", name, "")
	r["rules"] = cleanRules(rules)
	return r
}

func roleBinding(role, ns, sa string) map[string]any {
	b := obj("rbac.authorization.k8s.io/v1", "RoleBinding", role+"-binding", ns)
	b["roleRef"] = map[string]any{"apiGroup": "rbac.authorization.k8s.io", "kind": "Role", "name": role}
	b["subjects"] = []any{map[string]any{"kind": "ServiceAccount", "name": sa, "namespace": IdentityNamespace}}
	return b
}

func clusterRoleBinding(role, sa string) map[string]any {
	b := obj("rbac.authorization.k8s.io/v1", "ClusterRoleBinding", role+"-binding", "")
	b["roleRef"] = map[string]any{"apiGroup": "rbac.authorization.k8s.io", "kind": "ClusterRole", "name": role}
	b["subjects"] = []any{map[string]any{"kind": "ServiceAccount", "name": sa, "namespace": IdentityNamespace}}
	return b
}

// cleanRules converts rule maps to the plain array shape kubectl expects.
func cleanRules(rules []map[string]any) []any {
	out := make([]any, 0, len(rules))
	for _, r := range rules {
		out = append(out, r)
	}
	return out
}

func obj(apiVersion, kind, name, ns string) map[string]any {
	m := map[string]any{"apiVersion": apiVersion, "kind": kind, "metadata": map[string]any{"name": name}}
	meta := m["metadata"].(map[string]any)
	if ns != "" {
		meta["namespace"] = ns
	}
	return m
}

func strList(in []string) []any {
	out := make([]any, 0, len(in))
	for _, s := range in {
		out = append(out, s)
	}
	return out
}

func strContains(list []string, s string) bool {
	for _, e := range list {
		if e == s {
			return true
		}
	}
	return false
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// --- tokens ------------------------------------------------------------------

type clusterEndpoint struct {
	Server   string
	CAData   string
	Insecure bool
}

func (p *IdentityProvisioner) clusterInfo(ctx context.Context, adminKubeconfig string) (clusterEndpoint, error) {
	out, err := p.kctl(ctx, adminKubeconfig, "config", "view", "--minify", "--raw", "-o", "json")
	if err != nil {
		return clusterEndpoint{}, fmt.Errorf("identity: read admin kubeconfig: %w: %s", err, out)
	}
	var view struct {
		Clusters []struct {
			Name    string `json:"name"`
			Cluster struct {
				Server                   string `json:"server"`
				CertificateAuthorityData string `json:"certificate-authority-data"`
				InsecureSkipTLSVerify    string `json:"insecure-skip-tls-verify"`
			} `json:"cluster"`
		} `json:"clusters"`
	}
	if err := json.Unmarshal(out, &view); err != nil {
		return clusterEndpoint{}, fmt.Errorf("identity: parse admin kubeconfig: %w", err)
	}
	if len(view.Clusters) == 0 || view.Clusters[0].Cluster.Server == "" {
		return clusterEndpoint{}, fmt.Errorf("identity: admin kubeconfig has no cluster server")
	}
	c := view.Clusters[0].Cluster
	return clusterEndpoint{
		Server:   c.Server,
		CAData:   c.CertificateAuthorityData,
		Insecure: c.InsecureSkipTLSVerify == "true",
	}, nil
}

// mintToken returns a bearer token for the identity SA: TokenRequest API
// (kubectl create token) when the server supports it, legacy service-account
// secret on older servers (host kubectl may be < 1.24).
func (p *IdentityProvisioner) mintToken(ctx context.Context, adminKubeconfig string, b *IdentityBundle, sa string) (string, error) {
	if out, err := p.kctl(ctx, adminKubeconfig, "create", "token", sa, "-n", IdentityNamespace, "--duration", tokenDurationReqs); err == nil {
		tok := strings.TrimSpace(string(out))
		if strings.Count(tok, ".") == 2 && tok != "" {
			return tok, nil
		}
		err = fmt.Errorf("unexpected create token output")
		_ = err
	}
	// Legacy fallback: dedicated SA token secret, populated by the
	// serviceaccount token controller.
	secretName := sa + "-evidra-legacy"
	manifest := obj("v1", "Secret", secretName, IdentityNamespace)
	manifest["type"] = "kubernetes.io/service-account-token"
	manifest["metadata"] = map[string]any{
		"name":      secretName,
		"namespace": IdentityNamespace,
		"annotations": map[string]any{
			"kubernetes.io/service-account.name": sa,
		},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("identity: marshal secret: %w", err)
	}
	path := filepath.Join(b.dir, "secret-"+sa+".json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	if out, err := p.kctl(ctx, adminKubeconfig, "apply", "-f", path); err != nil {
		return "", fmt.Errorf("identity: legacy token secret: %w: %s", err, out)
	}
	b.files = append(b.files, path)
	for attempt := 0; attempt < 30; attempt++ {
		out, err := p.kctl(ctx, adminKubeconfig, "get", "secret", secretName, "-n", IdentityNamespace, "-o", "jsonpath={.data.token}")
		if err == nil && len(strings.TrimSpace(string(out))) > 0 {
			tok, derr := base64.StdEncoding.DecodeString(strings.TrimSpace(string(out)))
			if derr == nil && len(tok) > 0 {
				return string(tok), nil
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return "", fmt.Errorf("identity: legacy token secret %s never populated", secretName)
}

func writeKubeconfig(path string, c clusterEndpoint, token string) error {
	cluster := map[string]any{"server": c.Server}
	if c.Insecure {
		cluster["insecure-skip-tls-verify"] = true
	} else {
		cluster["certificate-authority-data"] = c.CAData
	}
	kc := map[string]any{
		"apiVersion":      "v1",
		"kind":            "Config",
		"clusters":        []any{map[string]any{"name": "evidra", "cluster": cluster}},
		"contexts":        []any{map[string]any{"name": "evidra", "context": map[string]any{"cluster": "evidra", "user": "evidra"}}},
		"current-context": "evidra",
		"users":           []any{map[string]any{"name": "evidra", "user": map[string]any{"token": token}}},
	}
	data, err := json.Marshal(kc)
	if err != nil {
		return fmt.Errorf("identity: marshal kubeconfig: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

// --- identity_auth_ready gate -------------------------------------------------

// WaitForAuthReady implements the mandatory identity_auth_ready gate
// (plan Phase 4; spike finding: on kind, bearer auth can be answered as
// system:anonymous for minutes after API start, regardless of token type).
// Ready means the API answered a probe with the *identity* attributed: an
// authenticated 403 (Forbidden naming the SA) or a 404 on the probe object.
func (p *IdentityProvisioner) WaitForAuthReady(ctx context.Context, id *Identity, timeout time.Duration) error {
	if id == nil {
		return nil
	}
	deadline := time.Now().Add(timeout)
	nonce := randNonce()
	probeName := probeNamePrefix + nonce
	var last string
	for {
		out, err := p.kctl(ctx, id.KubeconfigPath, "get", "configmap", probeName, "-n", IdentityNamespace)
		msg := strings.TrimSpace(string(out))
		if err == nil {
			return nil // granted read (shouldn't be, but auth works)
		}
		last = msg
		if strings.Contains(msg, "system:anonymous") {
			// Cold window: the token was not honored yet.
		} else if strings.Contains(msg, "Forbidden") || strings.Contains(msg, "not found") {
			return nil // authenticated as the SA — gate satisfied
		} else if isTransportFailure(msg) {
			return fmt.Errorf("identity_auth_ready(%s): cluster unreachable: %s", id.User, truncate(msg, 400))
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("identity_auth_ready(%s): timed out after %s; last: %s", id.User, timeout, truncate(last, 400))
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("identity_auth_ready(%s): %w", id.User, ctx.Err())
		case <-time.After(3 * time.Second):
		}
	}
}

func isTransportFailure(msg string) bool {
	l := strings.ToLower(msg)
	for _, m := range []string{"connection refused", "no such host", "dial tcp", "i/o timeout", "couldn't get current server api"} {
		if strings.Contains(l, m) {
			return true
		}
	}
	return false
}

// --- window markers (harness certificate identity) ----------------------------

// EmitMarker performs the spike-proven window marker: a GET of a
// nonce-named ConfigMap using the harness client-certificate identity. The
// resulting (authenticated) 404 audit event IS the marker; there is nothing
// to create and no bearer token involved, so markers work through the cold
// window. Returns the marker name (for audit correlation).
func (p *IdentityProvisioner) EmitMarker(ctx context.Context, adminKubeconfig string) (string, error) {
	name := markerNamePrefix + randNonce()
	out, err := p.kctl(ctx, adminKubeconfig, "get", "configmap", name, "-n", IdentityNamespace)
	if err == nil {
		return name, nil // object unexpectedly exists; event still audited
	}
	msg := strings.TrimSpace(string(out))
	if strings.Contains(msg, "not found") {
		return name, nil
	}
	if strings.Contains(msg, "system:anonymous") {
		return name, fmt.Errorf("marker: admin cert authenticated as system:anonymous: %s", truncate(msg, 200))
	}
	if isTransportFailure(msg) {
		return name, fmt.Errorf("marker: cluster unreachable: %s", truncate(msg, 200))
	}
	// RBAC-shaped denial still produces an attributed audit event; the
	// identity is still proven.
	if strings.Contains(msg, "Forbidden") {
		return name, nil
	}
	return name, fmt.Errorf("marker: unexpected probe failure: %s", truncate(msg, 200))
}

func randNonce() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%016x", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
