package environment

import (
	"os"
	"path/filepath"
	"testing"

	"samebits.com/evidra-infra-bench/pkg/scenario"
)

func TestAssetResolver_ResolveDefaultProfile(t *testing.T) {
	t.Parallel()

	root := setupAssetTree(t)
	r := AssetResolver{RootDir: root}

	assets, err := r.Resolve("kind", scenario.ProfileDefault)
	if err != nil {
		t.Fatalf("Resolve(kind, default) error: %v", err)
	}

	wantCluster := filepath.Join(root, "clusters", "kind", "default.yaml")
	if assets.ClusterConfigPath != wantCluster {
		t.Errorf("ClusterConfigPath = %q, want %q", assets.ClusterConfigPath, wantCluster)
	}

	wantDir := filepath.Join(root, "profiles", "default")
	if assets.ProfileDir != wantDir {
		t.Errorf("ProfileDir = %q, want %q", assets.ProfileDir, wantDir)
	}

	// Default profile has no hooks.
	if assets.InstallScript != "" {
		t.Errorf("InstallScript = %q, want empty for default profile", assets.InstallScript)
	}
	if assets.HealthcheckScript != "" {
		t.Errorf("HealthcheckScript = %q, want empty for default profile", assets.HealthcheckScript)
	}
	if assets.CleanupScript != "" {
		t.Errorf("CleanupScript = %q, want empty for default profile", assets.CleanupScript)
	}
}

func TestAssetResolver_ResolveArgocdProfile(t *testing.T) {
	t.Parallel()

	root := setupAssetTree(t)
	r := AssetResolver{RootDir: root}

	assets, err := r.Resolve("kind", scenario.ProfileArgocd)
	if err != nil {
		t.Fatalf("Resolve(kind, argocd) error: %v", err)
	}

	wantCluster := filepath.Join(root, "clusters", "kind", "argocd.yaml")
	if assets.ClusterConfigPath != wantCluster {
		t.Errorf("ClusterConfigPath = %q, want %q", assets.ClusterConfigPath, wantCluster)
	}

	wantInstall := filepath.Join(root, "profiles", "argocd", "install.sh")
	if assets.InstallScript != wantInstall {
		t.Errorf("InstallScript = %q, want %q", assets.InstallScript, wantInstall)
	}

	wantHealthcheck := filepath.Join(root, "profiles", "argocd", "healthcheck.sh")
	if assets.HealthcheckScript != wantHealthcheck {
		t.Errorf("HealthcheckScript = %q, want %q", assets.HealthcheckScript, wantHealthcheck)
	}

	wantCleanup := filepath.Join(root, "profiles", "argocd", "cleanup.sh")
	if assets.CleanupScript != wantCleanup {
		t.Errorf("CleanupScript = %q, want %q", assets.CleanupScript, wantCleanup)
	}
}

func TestAssetResolver_ResolveAWSLocalStackProfile(t *testing.T) {
	t.Parallel()

	root := setupAssetTree(t)
	r := AssetResolver{RootDir: root}

	assets, err := r.Resolve("k3d", scenario.ProfileAWSLocalStack)
	if err != nil {
		t.Fatalf("Resolve(k3d, aws-localstack) error: %v", err)
	}

	wantCluster := filepath.Join(root, "clusters", "k3d", "aws-localstack.yaml")
	if assets.ClusterConfigPath != wantCluster {
		t.Errorf("ClusterConfigPath = %q, want %q", assets.ClusterConfigPath, wantCluster)
	}

	wantInstall := filepath.Join(root, "profiles", "aws-localstack", "install.sh")
	if assets.InstallScript != wantInstall {
		t.Errorf("InstallScript = %q, want %q", assets.InstallScript, wantInstall)
	}

	// Healthcheck present.
	wantHealthcheck := filepath.Join(root, "profiles", "aws-localstack", "healthcheck.sh")
	if assets.HealthcheckScript != wantHealthcheck {
		t.Errorf("HealthcheckScript = %q, want %q", assets.HealthcheckScript, wantHealthcheck)
	}

	// Cleanup present.
	wantCleanup := filepath.Join(root, "profiles", "aws-localstack", "cleanup.sh")
	if assets.CleanupScript != wantCleanup {
		t.Errorf("CleanupScript = %q, want %q", assets.CleanupScript, wantCleanup)
	}
}

func TestAssetResolver_RejectsMissingNonDefaultInstallHook(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// Create cluster config but no profile install script.
	clusterDir := filepath.Join(root, "clusters", "kind")
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clusterDir, "argocd.yaml"), []byte("# empty"), 0o644); err != nil {
		t.Fatal(err)
	}
	profileDir := filepath.Join(root, "profiles", "argocd")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// No install.sh created.

	r := AssetResolver{RootDir: root}
	_, err := r.Resolve("kind", scenario.ProfileArgocd)
	if err == nil {
		t.Fatal("expected error for missing install.sh, got nil")
	}
}

// setupAssetTree creates a temporary asset tree that mirrors the real
// clusters/ and profiles/ layout. It is shared by the happy-path tests.
func setupAssetTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	dirs := []string{
		"clusters/kind",
		"clusters/k3d",
		"profiles/default",
		"profiles/argocd",
		"profiles/aws-localstack",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	files := []string{
		"clusters/kind/default.yaml",
		"clusters/kind/argocd.yaml",
		"clusters/kind/aws-localstack.yaml",
		"clusters/k3d/default.yaml",
		"clusters/k3d/argocd.yaml",
		"clusters/k3d/aws-localstack.yaml",
		"profiles/argocd/install.sh",
		"profiles/argocd/healthcheck.sh",
		"profiles/argocd/cleanup.sh",
		"profiles/aws-localstack/install.sh",
		"profiles/aws-localstack/healthcheck.sh",
		"profiles/aws-localstack/cleanup.sh",
	}
	for _, f := range files {
		path := filepath.Join(root, f)
		if err := os.WriteFile(path, []byte("# test stub"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	return root
}
