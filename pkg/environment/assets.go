package environment

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/vitas/evidra-bench/pkg/scenario"
)

// ProfileAssets holds resolved filesystem paths for a provider+profile combination.
// All paths are absolute so downstream code does not depend on working directory.
type ProfileAssets struct {
	// ClusterConfigPath is the provider-specific cluster configuration file.
	ClusterConfigPath string

	// ProfileDir is the root of the profile's hook directory.
	ProfileDir string

	// InstallScript is the path to install.sh, empty for the default profile.
	InstallScript string

	// HealthcheckScript is the path to healthcheck.sh, empty if not present.
	HealthcheckScript string

	// CleanupScript is the path to cleanup.sh, empty if not present.
	CleanupScript string
}

// AssetResolver maps a provider and execution profile to filesystem paths
// under a checked-in asset tree. RootDir must be the repository root
// containing the clusters/ and profiles/ directories.
type AssetResolver struct {
	RootDir string
}

// Resolve returns the asset paths for the given provider and profile.
// It requires clusters/<provider>/<profile>.yaml to exist. For non-default
// profiles it also requires profiles/<profile>/install.sh to exist.
// Healthcheck and cleanup scripts are optional.
func (r AssetResolver) Resolve(provider string, profile scenario.ExecutionProfile) (ProfileAssets, error) {
	if profile == "" {
		profile = scenario.ProfileDefault
	}

	clusterConfig := filepath.Join(r.RootDir, "clusters", provider, string(profile)+".yaml")
	if _, err := os.Stat(clusterConfig); err != nil {
		return ProfileAssets{}, fmt.Errorf("asset resolver: cluster config not found: %s", clusterConfig)
	}

	profileDir := filepath.Join(r.RootDir, "profiles", string(profile))

	var assets ProfileAssets
	assets.ClusterConfigPath = clusterConfig
	assets.ProfileDir = profileDir

	if profile == scenario.ProfileDefault {
		return assets, nil
	}

	// Non-default profiles require an install script.
	installScript := filepath.Join(profileDir, "install.sh")
	if _, err := os.Stat(installScript); err != nil {
		return ProfileAssets{}, fmt.Errorf("asset resolver: install.sh required for profile %q: %s", profile, installScript)
	}
	assets.InstallScript = installScript

	// Healthcheck and cleanup are optional.
	healthcheck := filepath.Join(profileDir, "healthcheck.sh")
	if _, err := os.Stat(healthcheck); err == nil {
		assets.HealthcheckScript = healthcheck
	}

	cleanup := filepath.Join(profileDir, "cleanup.sh")
	if _, err := os.Stat(cleanup); err == nil {
		assets.CleanupScript = cleanup
	}

	return assets, nil
}
