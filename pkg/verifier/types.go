// Package verifier evaluates scenario outcome quality.
package verifier

import "context"

// Verdict is the outcome of a verification check.
type Verdict string

const (
	VerdictPass Verdict = "pass"
	VerdictFail Verdict = "fail"
	VerdictSkip Verdict = "skip"
)

// CheckResult holds the result of a single verification check.
type CheckResult struct {
	Name    string  `json:"name"`
	Type    string  `json:"type"`
	Verdict Verdict `json:"verdict"`
	Message string  `json:"message,omitempty"`
}

// VerifyResult holds the aggregate verification result.
type VerifyResult struct {
	Passed bool          `json:"passed"`
	Checks []CheckResult `json:"checks"`
}

// Checker runs a single verification check against the cluster.
type Checker interface {
	Check(ctx context.Context, kubeconfigPath string) CheckResult
}
