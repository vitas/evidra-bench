package environment

import "samebits.com/evidra-infra-bench/pkg/scenario"

// ClusterSpec describes what cluster to create. Providers check ConfigPath
// first (checked-in asset file), then fall back to LegacyKubernetes (generated
// config from scenario YAML).
type ClusterSpec struct {
	// Profile is the execution profile that triggered this cluster creation.
	Profile scenario.ExecutionProfile

	// ConfigPath is the absolute path to a checked-in provider config file
	// (e.g. clusters/kind/default.yaml). When set, the provider uses this
	// file directly instead of generating configuration from Go code.
	ConfigPath string

	// LegacyKubernetes holds the old-style KubernetesConfig from scenario
	// YAML. Used as a fallback when ConfigPath is empty.
	LegacyKubernetes scenario.KubernetesConfig
}
