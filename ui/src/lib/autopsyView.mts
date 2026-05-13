export type AutopsyEvidence =
  | string
  | {
      artifact?: string;
      selector?: string;
      command?: string;
      text?: string;
      [key: string]: unknown;
    };

export interface AutopsyFinding {
  kind: string;
  severity: string;
  message: string;
  evidence?: AutopsyEvidence;
}

export interface AutopsyMetrics {
  turns: number;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  estimated_cost_usd: number;
  checks_passed: number;
  checks_total: number;
  mutation_count: number;
  diagnosis_depth: number;
  total_steps: number;
}

export interface AutopsyWaste {
  turns?: number;
  tokens?: number;
  basis?: string;
}

export interface AutopsyReport {
  version?: string;
  outcome: string;
  primary_failure?: string;
  summary: string;
  confidence?: string;
  findings?: AutopsyFinding[];
  metrics: AutopsyMetrics;
  wasted_turns?: number;
  wasted_tokens?: number;
  waste?: AutopsyWaste;
}

export interface AutopsyFindingView {
  kind: string;
  kindLabel: string;
  severity: string;
  message: string;
  evidenceText: string;
}

export interface AutopsyView {
  version: string;
  outcome: string;
  primaryFailure: string;
  primaryLabel: string;
  summary: string;
  confidence: string;
  findings: AutopsyFindingView[];
  metrics: AutopsyMetrics;
  waste: {
    turns: number;
    tokens: number;
    basis: string;
  };
}

export function formatFailureKind(kind: string | undefined): string {
  if (!kind) return "none";
  return kind
    .split("_")
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

export function normalizeAutopsyReport(report: AutopsyReport): AutopsyView {
  const primaryFailure =
    report.primary_failure || (report.outcome === "pass" ? "none" : "");

  return {
    version: report.version || "legacy",
    outcome: report.outcome,
    primaryFailure,
    primaryLabel: formatFailureKind(primaryFailure),
    summary: report.summary,
    confidence: report.confidence || "unknown",
    metrics: report.metrics,
    findings: (report.findings || []).map((finding) => ({
      kind: finding.kind,
      kindLabel: formatFailureKind(finding.kind),
      severity: finding.severity,
      message: finding.message,
      evidenceText: formatEvidence(finding.evidence),
    })),
    waste: {
      turns: report.waste?.turns ?? report.wasted_turns ?? 0,
      tokens: report.waste?.tokens ?? report.wasted_tokens ?? 0,
      basis: report.waste?.basis ?? "",
    },
  };
}

export function formatEvidence(evidence: AutopsyEvidence | undefined): string {
  if (!evidence) return "";
  if (typeof evidence === "string") return evidence;

  const parts = [
    evidence.artifact,
    evidence.selector,
    evidence.command,
    evidence.text,
  ].filter((part): part is string => Boolean(part));

  if (parts.length > 0) {
    return parts.join(" ");
  }

  return JSON.stringify(evidence);
}
