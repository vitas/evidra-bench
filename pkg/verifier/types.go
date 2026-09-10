// Package verifier evaluates scenario outcome quality.
package verifier

import "context"

// Verdict is the outcome of a verification check.
type Verdict string

const (
	VerdictPass Verdict = "pass"
	VerdictFail Verdict = "fail"
	VerdictSkip Verdict = "skip"
	// VerdictError means the check could not be evaluated (evaluator
	// transport/timeout/parse/RBAC failure). It is NOT a behavioral fail:
	// the harness must map a run containing errored checks to INCOMPLETE
	// (evaluator_error), never to PASS or FAIL.
	VerdictError Verdict = "error"
)

// CheckError classifies why a check could not be evaluated.
type CheckError struct {
	// Kind is one of transport|timeout|parse|rbac|exit-code.
	Kind    string `json:"kind"`
	Message string `json:"message,omitempty"`
}

// CheckResult holds the result of a single verification check.
type CheckResult struct {
	Name    string  `json:"name"`
	Type    string  `json:"type"`
	Verdict Verdict `json:"verdict"`
	Message string  `json:"message,omitempty"`
	// Error is set exactly when Verdict == VerdictError.
	Error *CheckError `json:"error,omitempty"`
}

// VerifyResult holds the aggregate verification result.
type VerifyResult struct {
	Passed bool          `json:"passed"`
	Checks []CheckResult `json:"checks"`
}

// Errored returns the checks that failed to evaluate.
func (r *VerifyResult) Errored() []CheckResult {
	if r == nil {
		return nil
	}
	var out []CheckResult
	for _, c := range r.Checks {
		if c.Verdict == VerdictError {
			out = append(out, c)
		}
	}
	return out
}

// Checker runs a single verification check against the cluster.
type Checker interface {
	Check(ctx context.Context, kubeconfigPath string) CheckResult
}
