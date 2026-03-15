package skilldelta

// Benchmark is the machine-readable output for a skill-delta experiment run.
type Benchmark struct {
	Metadata BenchmarkMetadata `json:"metadata"`
	Pairs    []PairResult      `json:"pairs"`
	Summary  BenchmarkSummary  `json:"summary"`
}

// BenchmarkMetadata describes the benchmark suite and provenance information.
type BenchmarkMetadata struct {
	Suite             string   `json:"suite"`
	GeneratedAt       string   `json:"generated_at"`
	RunsDir           string   `json:"runs_dir,omitempty"`
	Repeats           int      `json:"repeats"`
	Provider          string   `json:"provider,omitempty"`
	Scenarios         []string `json:"scenarios,omitempty"`
	Models            []string `json:"models,omitempty"`
	NoSkillPrompt     string   `json:"no_skill_prompt,omitempty"`
	WithSkillPrompt   string   `json:"with_skill_prompt,omitempty"`
	InfraBenchVersion string   `json:"infra_bench_version,omitempty"`
	InfraBenchCommit  string   `json:"infra_bench_commit,omitempty"`
	EvidraVersion     string   `json:"evidra_version,omitempty"`
	ContractVersion   string   `json:"contract_version,omitempty"`
	PromptVersion     string   `json:"prompt_version,omitempty"`
	SkillVersion      string   `json:"skill_version,omitempty"`
}

// PairResult is one without-skill vs with-skill comparison for a single
// scenario/model/repeat tuple.
type PairResult struct {
	ScenarioID           string      `json:"scenario_id"`
	Model                string      `json:"model"`
	Provider             string      `json:"provider,omitempty"`
	Repeat               int         `json:"repeat"`
	WithoutSkill         RunSnapshot `json:"without_skill"`
	WithSkill            RunSnapshot `json:"with_skill"`
	VerdictDelta         string      `json:"verdict_delta"`
	DurationDeltaSeconds float64     `json:"duration_delta_seconds"`
	CostDeltaUSD         float64     `json:"cost_delta_usd"`
	ComplianceDeltaPct   float64     `json:"compliance_delta_pct"`
	ScoreDelta           float64     `json:"score_delta"`
	TokenDelta           TokenDelta  `json:"token_delta"`
	Paths                PairPaths   `json:"paths,omitempty"`
}

// RunSnapshot holds the normalized metrics for a single benchmark run.
type RunSnapshot struct {
	RunID            string            `json:"run_id,omitempty"`
	Passed           bool              `json:"passed"`
	ExitCode         int               `json:"exit_code,omitempty"`
	DurationSeconds  float64           `json:"duration_seconds"`
	Turns            int               `json:"turns"`
	PromptTokens     int               `json:"prompt_tokens"`
	CompletionTokens int               `json:"completion_tokens"`
	TotalTokens      int               `json:"total_tokens"`
	EstimatedCostUSD float64           `json:"estimated_cost_usd"`
	ChecksPassed     int               `json:"checks_passed"`
	ChecksTotal      int               `json:"checks_total"`
	Protocol         ProtocolMetrics   `json:"protocol"`
	Scorecard        ScorecardMetrics  `json:"scorecard"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

// ProtocolMetrics captures protocol-compliance specific counters for one run.
type ProtocolMetrics struct {
	PrescribeCount        int     `json:"prescribe_count"`
	ReportCount           int     `json:"report_count"`
	OrphanedPrescriptions int     `json:"orphaned_prescriptions"`
	DeclinedCount         int     `json:"declined_count"`
	ChecksPassed          int     `json:"checks_passed"`
	ChecksTotal           int     `json:"checks_total"`
	VerdictCoveragePct    float64 `json:"verdict_coverage_pct"`
	ComplianceRatePct     float64 `json:"compliance_rate_pct"`
}

// ScorecardMetrics captures scorecard output derived from Evidra evidence.
type ScorecardMetrics struct {
	Available    bool           `json:"available"`
	Score        *float64       `json:"score,omitempty"`
	Band         string         `json:"band,omitempty"`
	Signals      []string       `json:"signals,omitempty"`
	SignalCounts map[string]int `json:"signal_counts,omitempty"`
}

// TokenDelta captures the change in token use between configurations.
type TokenDelta struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// PairPaths links a pair result back to the underlying local artifacts.
type PairPaths struct {
	WithoutSkillRunDir      string `json:"without_skill_run_dir,omitempty"`
	WithSkillRunDir         string `json:"with_skill_run_dir,omitempty"`
	WithoutSkillTranscript  string `json:"without_skill_transcript,omitempty"`
	WithSkillTranscript     string `json:"with_skill_transcript,omitempty"`
	WithoutSkillEvidenceDir string `json:"without_skill_evidence_dir,omitempty"`
	WithSkillEvidenceDir    string `json:"with_skill_evidence_dir,omitempty"`
	WithoutSkillScorecard   string `json:"without_skill_scorecard,omitempty"`
	WithSkillScorecard      string `json:"with_skill_scorecard,omitempty"`
}

// BenchmarkSummary is the aggregate view of all pair results.
type BenchmarkSummary struct {
	PairCount    int                  `json:"pair_count"`
	WithoutSkill ConfigurationSummary `json:"without_skill"`
	WithSkill    ConfigurationSummary `json:"with_skill"`
	Delta        DeltaSummary         `json:"delta"`
}

// ConfigurationSummary aggregates one benchmark configuration across all pairs.
type ConfigurationSummary struct {
	PassRatePct      NumericSummary `json:"pass_rate_pct"`
	CompliancePct    NumericSummary `json:"compliance_pct"`
	DurationSeconds  NumericSummary `json:"duration_seconds"`
	TotalTokens      NumericSummary `json:"total_tokens"`
	EstimatedCostUSD NumericSummary `json:"estimated_cost_usd"`
	Score            NumericSummary `json:"score"`
}

// NumericSummary is a standard aggregate for one numeric metric.
type NumericSummary struct {
	Mean   float64 `json:"mean"`
	Stddev float64 `json:"stddev"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
}

// DeltaSummary stores configuration deltas as with-skill minus without-skill.
type DeltaSummary struct {
	PassRatePct      float64 `json:"pass_rate_pct"`
	CompliancePct    float64 `json:"compliance_pct"`
	DurationSeconds  float64 `json:"duration_seconds"`
	TotalTokens      float64 `json:"total_tokens"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
	Score            float64 `json:"score"`
}
