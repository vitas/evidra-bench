package environment

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// K3dProvider manages k3d cluster lifecycles.
type K3dProvider struct {
	kubectlOps
	ReuseExisting bool
	ContainerName string
	// NetworkMode is the detected runner container networking mode. The
	// zero value preserves legacy bridge behavior for containers.
	NetworkMode ContainerNetworkMode
}

// NewK3dProvider returns a K3dProvider with the default command runner.
func NewK3dProvider() *K3dProvider {
	runner := &ExecRunner{}
	provider := &K3dProvider{
		kubectlOps: kubectlOps{Runner: runner},
	}
	if os.Getenv("EVIDRA_CONTAINERIZED") == "1" {
		provider.ContainerName = strings.TrimSpace(os.Getenv("HOSTNAME"))
	}
	return provider
}

func (p *K3dProvider) createCommand(clusterName string) *exec.Cmd {
	return exec.Command("k3d", "cluster", "create", clusterName,
		"--no-lb",
		"--wait",
	)
}

// createCommandWithExplicitConfig builds a k3d create command that uses a
// checked-in config file directly.
func (p *K3dProvider) createCommandWithExplicitConfig(clusterName, configPath string) *exec.Cmd {
	return exec.Command("k3d", "cluster", "create", clusterName,
		"--config", configPath,
	)
}

func (p *K3dProvider) deleteCommand(clusterName string) *exec.Cmd {
	return exec.Command("k3d", "cluster", "delete", clusterName)
}

func (p *K3dProvider) listClustersCommand() *exec.Cmd {
	return exec.Command("k3d", "cluster", "list", "--no-headers")
}

func (p *K3dProvider) kubeconfigCommand(clusterName string) *exec.Cmd {
	return exec.Command("k3d", "kubeconfig", "get", clusterName)
}

func (p *K3dProvider) connectContainerCommand(clusterName string) *exec.Cmd {
	return exec.Command("docker", "network", "connect", "k3d-"+clusterName, p.ContainerName)
}

func (p *K3dProvider) disconnectContainerCommand(clusterName string) *exec.Cmd {
	return exec.Command("docker", "network", "disconnect", "k3d-"+clusterName, p.ContainerName)
}

// RunCanary uses k3s's preloaded sandbox image as a fallback because k3s does
// not expose that small image through Node.status.images on a fresh cluster.
func (p *K3dProvider) RunCanary(ctx context.Context, kubeconfigPath, ns string) error {
	return p.runCanary(ctx, kubeconfigPath, ns, "docker.io/rancher/mirrored-pause:3.6")
}

// Create provisions a new k3d cluster and writes a kubeconfig file.
// When spec.ConfigPath is set, uses the checked-in config file.
// Otherwise, creates a plain cluster (LegacyKubernetes options are not
// supported by k3d and are logged as warnings).
func (p *K3dProvider) Create(ctx context.Context, clusterName string, spec ClusterSpec) (*Handle, error) {
	k8s := spec.LegacyKubernetes
	if spec.ConfigPath == "" && (k8s.CNI != "" || len(k8s.Addons) > 0 || len(k8s.Runtimes) > 0 || len(k8s.Features) > 0) {
		log.Printf("[k3d] warning: KubernetesConfig options not supported by k3d provider")
	}
	exists, err := p.clusterExists(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("environment.K3dProvider.Create: check existing cluster: %w", err)
	}
	if !exists || !p.ReuseExisting {
		if exists {
			// Delete first — k3d can't create over an existing cluster.
			delCmd := p.deleteCommand(clusterName)
			if out, err := p.Runner.Run(ctx, delCmd); err != nil {
				return nil, fmt.Errorf("environment.K3dProvider.Create: delete existing cluster: %w: %s", err, string(out))
			}
		}
		var cmd *exec.Cmd
		switch {
		case spec.Audit.Enabled:
			if _, err := StageAuditVolume(ctx, p.Runner, clusterName, AuditPolicyYAML(spec.Audit.PolicyMode)); err != nil {
				return nil, err
			}
			base := []string{"cluster", "create", clusterName}
			if spec.ConfigPath != "" {
				// k3d merges --config with additive CLI flags; volumes and
				// k3s args extend the asset rather than replace it.
				base = append(base, "--config", spec.ConfigPath)
			} else {
				base = append(base, "--no-lb", "--wait")
			}
			args := append(base, K3dAuditArgs(auditVolumeName(clusterName))...)
			cmd = exec.Command("k3d", args...)
		default:
			if spec.ConfigPath != "" {
				cmd = p.createCommandWithExplicitConfig(clusterName, spec.ConfigPath)
			} else {
				cmd = p.createCommand(clusterName)
			}
		}
		if out, err := p.Runner.Run(ctx, cmd); err != nil {
			if spec.Audit.Enabled {
				_ = RemoveAuditVolume(ctx, p.Runner, clusterName)
			}
			return nil, fmt.Errorf("environment.K3dProvider.Create: %w: %s", err, strings.TrimSpace(string(out)))
		}
		if spec.Audit.Enabled {
			// k3s only writes the audit log once the directory exists
			// inside the server container (proven recipe).
			mkdir := exec.Command("docker", "exec", "k3d-"+clusterName+"-server-0", "mkdir", "-p", "/var/log/kubernetes")
			if out, err := p.Runner.Run(ctx, mkdir); err != nil {
				log.Printf("[k3d] audit log dir prep failed (collector will surface coverage): %v: %s", err, strings.TrimSpace(string(out)))
			}
		}
	}
	if p.ContainerName != "" && p.NetworkMode != ContainerNetworkHost {
		out, connectErr := p.Runner.Run(ctx, p.connectContainerCommand(clusterName))
		alreadyConnected := strings.Contains(strings.ToLower(string(out)), "already exists") || strings.Contains(strings.ToLower(string(out)), "already connected")
		if connectErr != nil && !alreadyConnected {
			if !exists {
				_, _ = p.Runner.Run(ctx, p.deleteCommand(clusterName))
			}
			return nil, fmt.Errorf("environment.K3dProvider.Create: connect runner container to k3d network: %w: %s", connectErr, strings.TrimSpace(string(out)))
		}
	}

	kubeconfigCmd := p.kubeconfigCommand(clusterName)
	out, err := p.Runner.Run(ctx, kubeconfigCmd)
	if err != nil {
		return nil, fmt.Errorf("environment.K3dProvider.Create: get kubeconfig: %w", err)
	}
	if p.ContainerName != "" {
		if p.NetworkMode == ContainerNetworkHost {
			// Host-network runners share the host namespace: keep the
			// published dynamic port, normalize only the host.
			out, err = localizeK3dKubeconfigServer(out)
		} else {
			out, err = rewriteK3dKubeconfigServer(out, clusterName)
		}
		if err != nil {
			return nil, fmt.Errorf("environment.K3dProvider.Create: prepare container kubeconfig: %w", err)
		}
	}

	kubeconfigPath := filepath.Join(os.TempDir(), fmt.Sprintf("bench-cli-%s-kubeconfig", clusterName))
	if err := os.WriteFile(kubeconfigPath, out, 0600); err != nil {
		return nil, fmt.Errorf("environment.K3dProvider.Create: write kubeconfig: %w", err)
	}

	handle := &Handle{
		ClusterName:    clusterName,
		KubeconfigPath: kubeconfigPath,
		ClusterNetwork: "k3d-" + clusterName,
	}
	if spec.Audit.Enabled {
		handle.Audit = &AuditAccess{
			NodeContainers: []string{"k3d-" + clusterName + "-server-0"},
			LogPath:        "/var/log/kubernetes/audit.log",
			MarkerUsername: "system:admin",
		}
	}
	return handle, nil
}

// Recreate tears down and re-creates the k3d cluster.
func (p *K3dProvider) Recreate(ctx context.Context, clusterName string, spec ClusterSpec) (*Handle, error) {
	log.Printf("[k3d] recreating cluster %s", clusterName)
	var disconnectErr error
	if p.ContainerName != "" && p.NetworkMode != ContainerNetworkHost {
		disconnectErr = p.disconnectContainer(ctx, clusterName)
	}
	delCmd := p.deleteCommand(clusterName)
	if out, err := p.Runner.Run(ctx, delCmd); err != nil {
		deleteErr := fmt.Errorf("environment.K3dProvider.Recreate: delete existing cluster: %w: %s", err, string(out))
		return nil, errors.Join(disconnectErr, deleteErr)
	}
	if disconnectErr != nil {
		return nil, disconnectErr
	}
	return p.Create(ctx, clusterName, spec)
}

func (p *K3dProvider) clusterExists(ctx context.Context, clusterName string) (bool, error) {
	cmd := p.listClustersCommand()
	out, err := p.Runner.Run(ctx, cmd)
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == clusterName {
			return true, nil
		}
	}
	return false, nil
}

// Destroy tears down the k3d cluster, its audit staging volume, and the
// kubeconfig file.
func (p *K3dProvider) Destroy(ctx context.Context, handle *Handle) error {
	var disconnectErr error
	if p.ContainerName != "" && p.NetworkMode != ContainerNetworkHost {
		disconnectErr = p.disconnectContainer(ctx, handle.ClusterName)
	}
	cmd := p.deleteCommand(handle.ClusterName)
	if _, err := p.Runner.Run(ctx, cmd); err != nil {
		return errors.Join(disconnectErr, fmt.Errorf("environment.K3dProvider.Destroy: %w", err))
	}
	if err := RemoveAuditVolume(ctx, p.Runner, handle.ClusterName); err != nil {
		log.Printf("[k3d] audit volume cleanup for %s: %v", handle.ClusterName, err)
	}
	_ = os.Remove(handle.KubeconfigPath)
	return disconnectErr
}

func (p *K3dProvider) disconnectContainer(ctx context.Context, clusterName string) error {
	out, err := p.Runner.Run(ctx, p.disconnectContainerCommand(clusterName))
	if err == nil {
		return nil
	}
	message := strings.ToLower(string(out))
	if strings.Contains(message, "not connected") || strings.Contains(message, "no such container") || strings.Contains(message, "no such network") {
		return nil
	}
	return fmt.Errorf("disconnect runner container from k3d network: %w: %s", err, strings.TrimSpace(string(out)))
}

func rewriteK3dKubeconfigServer(data []byte, clusterName string) ([]byte, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}
	if len(document.Content) == 0 {
		return nil, fmt.Errorf("kubeconfig is empty")
	}
	root := document.Content[0]
	clusters := yamlMapValue(root, "clusters")
	if clusters == nil || clusters.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("kubeconfig has no clusters")
	}

	rewritten := 0
	for _, entry := range clusters.Content {
		cluster := yamlMapValue(entry, "cluster")
		server := yamlMapValue(cluster, "server")
		if server == nil || server.Kind != yaml.ScalarNode {
			continue
		}
		server.Value = fmt.Sprintf("https://k3d-%s-server-0:6443", clusterName)
		rewritten++
	}
	if rewritten == 0 {
		return nil, fmt.Errorf("kubeconfig has no cluster server")
	}
	return yaml.Marshal(&document)
}

// localizeK3dKubeconfigServer rewrites only the host component of every
// cluster server URL to 127.0.0.1, preserving the dynamically published API
// port and all credentials. Used when the runner shares the host network
// namespace, where k3d may emit host.docker.internal or 0.0.0.0.
func localizeK3dKubeconfigServer(data []byte) ([]byte, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}
	if len(document.Content) == 0 {
		return nil, fmt.Errorf("kubeconfig is empty")
	}
	clusters := yamlMapValue(document.Content[0], "clusters")
	if clusters == nil || clusters.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("kubeconfig has no clusters")
	}
	rewritten := 0
	for _, entry := range clusters.Content {
		server := yamlMapValue(yamlMapValue(entry, "cluster"), "server")
		if server == nil || server.Kind != yaml.ScalarNode {
			continue
		}
		parsed, err := url.Parse(server.Value)
		if err != nil {
			return nil, fmt.Errorf("kubeconfig server %q: %w", server.Value, err)
		}
		if parsed.Port() == "" {
			return nil, fmt.Errorf("kubeconfig server %q has no explicit API port", server.Value)
		}
		parsed.Host = "127.0.0.1:" + parsed.Port()
		server.Value = parsed.String()
		rewritten++
	}
	if rewritten == 0 {
		return nil, fmt.Errorf("kubeconfig has no cluster server")
	}
	return yaml.Marshal(&document)
}

func yamlMapValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return mapping.Content[index+1]
		}
	}
	return nil
}
