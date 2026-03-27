package scenario

// ExecutionProfile identifies the infrastructure contract a scenario requires.
// The provisioner uses this to select the correct cluster setup and addons.
type ExecutionProfile string

const (
	// ProfileDefault is the standard kind/k3d cluster with no extra addons.
	ProfileDefault ExecutionProfile = "default"

	// ProfileArgocd requires a cluster with ArgoCD pre-installed.
	ProfileArgocd ExecutionProfile = "argocd"

	// ProfileAWSLocalStack requires a cluster plus a running LocalStack instance.
	ProfileAWSLocalStack ExecutionProfile = "aws-localstack"
)

// supportedExecutionProfiles is the set of valid execution profile values.
var supportedExecutionProfiles = map[ExecutionProfile]bool{
	ProfileDefault:       true,
	ProfileArgocd:        true,
	ProfileAWSLocalStack: true,
}

// IsSupportedExecutionProfile reports whether p is a known execution profile.
func IsSupportedExecutionProfile(p ExecutionProfile) bool {
	return supportedExecutionProfiles[p]
}

// ResolvedProfile returns the effective execution profile for the scenario.
// Resolution order:
//  1. Explicit environment.profile wins.
//  2. Legacy environment.cloud.provider == "localstack" resolves to aws-localstack.
//  3. Otherwise default.
//
// Category, tags, and bootstrap contents are never used for inference.
func (s *Scenario) ResolvedProfile() ExecutionProfile {
	if s.Environment.Profile != "" {
		return s.Environment.Profile
	}
	if s.Environment.Cloud.Provider == "localstack" {
		return ProfileAWSLocalStack
	}
	return ProfileDefault
}
