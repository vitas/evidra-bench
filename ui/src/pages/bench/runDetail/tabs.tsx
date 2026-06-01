import type { ReactElement } from "react";
import { Link } from "react-router";
import { normalizeAutopsyReport, type AutopsyReport } from "../../../lib/autopsyView.mts";
import { formatCompactTokens } from "../../../lib/benchFormatters.mts";
import type { BenchRunRecord } from "../../../lib/benchTypes.mts";
import { normalizeRunReviewView, type RunReview } from "../../../lib/runReview.mts";
import { reviewDraftAvailable } from "../../../lib/runReviewEditor.mts";
import {
  scenarioPatchValidationProgress,
  scenarioPatchValidationRunIDs,
  scenarioPatchValidationStatus,
  scenarioPatchPreviewDiffFilename,
  scenarioPatchPreviewDownloadHref,
  scenarioPatchPreviewStatus,
  type ScenarioPatchPreview,
  type ScenarioPatchValidation,
} from "../../../lib/scenarioPatchPreview.mts";
import { benchRunPath } from "../../../lib/routes.mts";
import { RunReviewEditor } from "../RunReviewEditor";
import type { Scorecard, TimelineData, ToolCall } from "./types";

const PHASE_STYLES: Record<string, string> = {
  discover: "text-blue-400 bg-blue-400/10",
  diagnose: "text-purple-400 bg-purple-400/10",
  decide: "text-amber-400 bg-amber-400/10",
  act: "text-green-400 bg-green-400/10",
  verify: "text-teal-400 bg-teal-400/10",
  explain: "text-gray-400 bg-gray-400/10",
};

const PHASE_ORDER = ["discover", "diagnose", "decide", "act", "verify", "explain"];
const API_BASE = import.meta.env.VITE_BENCH_API_URL || "";

interface Check {
  name: string;
  type: string;
  verdict: string;
  message: string;
}

export interface ChecksPayload {
  passed: boolean;
  checks: Check[];
}

function truncate(s: string, max: number): string {
  if (s.length <= max) return s;
  return s.slice(0, max) + "\u2026";
}

function highlightTranscript(text: string): (ReactElement | string)[] {
  const parts = text.split(/(\[(?:user|assistant|tool)\])/g);
  return parts.map((part, i) => {
    if (part === "[user]") {
      return (
        <span key={i} className="text-info font-semibold">
          {part}
        </span>
      );
    }
    if (part === "[assistant]") {
      return (
        <span key={i} className="text-accent font-semibold">
          {part}
        </span>
      );
    }
    if (part === "[tool]") {
      return (
        <span key={i} className="text-warning font-semibold">
          {part}
        </span>
      );
    }
    return part;
  });
}

export function MetaItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-bg-alt/80 rounded-lg px-3.5 py-2.5">
      <div className="text-[0.68rem] font-semibold text-fg-muted uppercase tracking-wide">
        {label}
      </div>
      <div className="font-mono text-[0.85rem] font-semibold text-fg mt-0.5">
        {value}
      </div>
    </div>
  );
}

export function parseChecks(checksJson: string): ChecksPayload | null {
  if (!checksJson) return null;
  try {
    return JSON.parse(checksJson) as ChecksPayload;
  } catch {
    return null;
  }
}

export function SummaryTab({
  checks,
  scorecard,
}: {
  checks: ChecksPayload | null;
  scorecard: Scorecard | null;
}) {
  return (
    <div className="space-y-6">
      <div>
        <h3 className="text-[0.9rem] font-semibold text-fg mb-3">
          Verification Checks
        </h3>
        {checks && checks.checks && checks.checks.length > 0 ? (
          <div className="space-y-1.5">
            {checks.checks.map((c, i) => (
              <div
                key={i}
                className="flex items-center gap-3 px-3 py-2 bg-bg-alt/80 rounded-md text-[0.8rem]"
              >
                <span
                  className={`inline-block w-2 h-2 rounded-full flex-shrink-0 ${
                    c.verdict === "pass" ? "bg-accent" : "bg-danger"
                  }`}
                />
                <span className="font-mono text-fg-muted text-[0.75rem] min-w-[140px]">
                  {c.type}
                </span>
                <span className="text-fg">{c.name}</span>
                {c.message && (
                  <span className="text-fg-muted ml-auto text-[0.75rem] truncate max-w-[300px]">
                    {c.message}
                  </span>
                )}
              </div>
            ))}
          </div>
        ) : (
          <p className="text-fg-muted text-[0.82rem]">No checks recorded.</p>
        )}
      </div>

      <div>
        <h3 className="text-[0.9rem] font-semibold text-fg mb-3">
          Signals Detected
        </h3>
        {scorecard && scorecard.signals && Object.keys(scorecard.signals).length > 0 ? (
          <div className="space-y-1.5">
            {Object.entries(scorecard.signals).map(([name, count]) => (
              <div
                key={name}
                className="flex items-center gap-3 px-3 py-2 bg-bg-alt/80 rounded-md text-[0.8rem]"
              >
                <span className="font-mono text-fg">{name}</span>
                <span className="text-fg-muted ml-auto">&times;{count}</span>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-fg-muted text-[0.82rem]">
            No scorecard available. Run with scoring enabled to see signal data.
          </p>
        )}
      </div>
    </div>
  );
}

export function ReviewTab({
  run,
  review,
  timeline,
  loading,
  error,
  saving,
  saveError,
  saved,
  drafting,
  draftError,
  draftSeed,
  scenarioPatchPreview,
  scenarioPatchPreviewLoading,
  scenarioPatchPreviewError,
  scenarioPatchValidation,
  scenarioPatchValidationLoading,
  scenarioPatchValidationError,
  onDraft,
  onPreviewScenarioPatch,
  onValidateScenarioPatch,
  onRefreshScenarioPatchValidation,
  onSave,
}: {
  run: BenchRunRecord;
  review: RunReview | null;
  timeline: TimelineData | null;
  loading: boolean;
  error: string | null;
  saving: boolean;
  saveError: string | null;
  saved: boolean;
  drafting: boolean;
  draftError: string | null;
  draftSeed: RunReview | null;
  scenarioPatchPreview: ScenarioPatchPreview | null;
  scenarioPatchPreviewLoading: boolean;
  scenarioPatchPreviewError: string | null;
  scenarioPatchValidation: ScenarioPatchValidation | null;
  scenarioPatchValidationLoading: boolean;
  scenarioPatchValidationError: string | null;
  onDraft: () => Promise<RunReview>;
  onPreviewScenarioPatch: () => Promise<void>;
  onValidateScenarioPatch: () => Promise<void>;
  onRefreshScenarioPatchValidation: () => Promise<void>;
  onSave: (payload: RunReview) => Promise<void>;
}) {
  const canDraftReview = reviewDraftAvailable(run);

  if (loading) return <p className="text-fg-muted text-[0.82rem] py-6">Loading human review...</p>;
  if (error === "not-found" || (!review && !loading && !error)) {
    return (
      <div className="space-y-5">
        <div className="rounded-lg border border-border-subtle bg-bg-alt/60 p-4">
          <h3 className="text-[0.9rem] font-semibold text-fg mb-1">No human review yet</h3>
          <p className="text-fg-muted text-[0.82rem]">This run has no saved review artifact.</p>
        </div>
        <RunReviewEditor
          run={run}
          review={review}
          timeline={timeline}
          saving={saving}
          saveError={saveError}
          saved={saved}
          drafting={drafting}
          draftError={draftError}
          draftSeed={draftSeed}
          humanOnly={!canDraftReview}
          onDraft={canDraftReview ? onDraft : undefined}
          onSave={onSave}
        />
      </div>
    );
  }
  if (error) return <p className="text-danger text-[0.82rem] py-6">Failed to load human review: {error}</p>;
  if (!review) return <p className="text-fg-muted text-[0.82rem] py-6">No human review yet.</p>;

  const view = normalizeRunReviewView(review);
  const canPreviewScenarioPatch = (review.suggested_rules?.length ?? 0) > 0;

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <span className="inline-block px-2 py-0.5 rounded bg-accent-tint text-accent text-[0.7rem] font-semibold uppercase tracking-wide">
              {view.verdictLabel}
            </span>
            <span className="inline-block px-2 py-0.5 rounded bg-bg-alt/80 text-fg-muted text-[0.7rem] font-semibold uppercase tracking-wide">
              {view.visibilityLabel}
            </span>
            {review.primary_label && (
              <span className="inline-block px-2 py-0.5 rounded bg-warning/15 text-warning text-[0.7rem] font-mono">
                {review.primary_label}
              </span>
            )}
          </div>
          <p className="text-fg-muted text-[0.82rem] mt-2">
            Reviewed by {view.reviewerLabel}
          </p>
        </div>
        <span className="font-mono text-fg-muted text-[0.72rem]">
          {review.version || "run_review.v1"}
        </span>
      </div>

      <div>
        <h3 className="text-[0.9rem] font-semibold text-fg mb-3">Labels</h3>
        {view.labels.length > 0 ? (
          <div className="space-y-2">
            {view.labels.map((label, i) => (
              <div key={i} className="rounded-md bg-bg-alt/80 px-3 py-2.5 text-[0.8rem]">
                <div className="flex flex-wrap items-center gap-2">
                  <span className={`inline-block px-2 py-0.5 rounded text-[0.7rem] font-semibold uppercase tracking-wide ${severityStyle(label.severityLabel.toLowerCase())}`}>
                    {label.severityLabel}
                  </span>
                  <span className="font-mono text-fg">{label.kindLabel}</span>
                  {label.stepLabel && <span className="font-mono text-fg-muted text-[0.72rem]">{label.stepLabel}</span>}
                  {label.evidenceRefLabel && <span className="font-mono text-fg-muted text-[0.72rem]">{label.evidenceRefLabel}</span>}
                </div>
                {label.note && <p className="text-fg-muted mt-2 leading-relaxed">{label.note}</p>}
                {label.evidenceSnippet && (
                  <pre className="mt-2 rounded bg-code-bg border border-border-subtle px-3 py-2 font-mono text-[0.72rem] text-fg-muted whitespace-pre-wrap break-words">
                    {label.evidenceSnippet}
                  </pre>
                )}
              </div>
            ))}
          </div>
        ) : (
          <p className="text-fg-muted text-[0.82rem]">No labels saved.</p>
        )}
      </div>

      <div>
        <div className="mb-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <h3 className="text-[0.9rem] font-semibold text-fg">Suggested Rules</h3>
          {canPreviewScenarioPatch && (
            <button
              type="button"
              onClick={onPreviewScenarioPatch}
              disabled={scenarioPatchPreviewLoading}
              className="rounded-md border border-border bg-bg-alt px-3 py-1.5 text-[0.78rem] font-semibold text-fg transition-colors hover:border-accent disabled:cursor-default disabled:opacity-50"
            >
              {scenarioPatchPreviewLoading ? "Previewing..." : "Preview scenario patch"}
            </button>
          )}
        </div>
        {view.suggestedRules.length > 0 ? (
          <div className="space-y-1.5">
            {view.suggestedRules.map((rule, i) => (
              <div key={i} className="rounded-md bg-bg-alt/80 px-3 py-2 font-mono text-[0.76rem] text-fg-muted break-words">
                {rule}
              </div>
            ))}
          </div>
        ) : (
          <p className="text-fg-muted text-[0.82rem]">No scenario-rule suggestions saved with this review.</p>
        )}
        <ScenarioPatchPreviewPanel
          preview={scenarioPatchPreview}
          loading={scenarioPatchPreviewLoading}
          error={scenarioPatchPreviewError}
          validation={scenarioPatchValidation}
          validationLoading={scenarioPatchValidationLoading}
          validationError={scenarioPatchValidationError}
          onValidate={onValidateScenarioPatch}
          onRefreshValidation={onRefreshScenarioPatchValidation}
        />
      </div>

      <RunReviewEditor
        run={run}
        review={review}
        timeline={timeline}
        saving={saving}
        saveError={saveError}
        saved={saved}
        drafting={drafting}
        draftError={draftError}
        draftSeed={draftSeed}
        humanOnly={!canDraftReview}
        onDraft={canDraftReview ? onDraft : undefined}
        onSave={onSave}
      />
    </div>
  );
}

function ScenarioPatchPreviewPanel({
  preview,
  loading,
  error,
  validation,
  validationLoading,
  validationError,
  onValidate,
  onRefreshValidation,
}: {
  preview: ScenarioPatchPreview | null;
  loading: boolean;
  error: string | null;
  validation: ScenarioPatchValidation | null;
  validationLoading: boolean;
  validationError: string | null;
  onValidate: () => Promise<void>;
  onRefreshValidation: () => Promise<void>;
}) {
  if (loading) {
    return <div className="mt-3 rounded-md border border-border-subtle bg-bg-alt/50 px-3 py-2 text-[0.8rem] text-fg-muted">Building scenario patch preview...</div>;
  }
  if (error) {
    return <div className="mt-3 rounded-md bg-[var(--color-danger-badge-bg)] px-3 py-2 text-[0.8rem] text-[var(--color-danger-badge-fg)]">{error}</div>;
  }

  const validationProgress = validation ? scenarioPatchValidationProgress(validation) : null;
  const validationRunIDs = validation ? scenarioPatchValidationRunIDs(validation) : [];
  const validationPanel = validation ? (
    <div className="mb-2 rounded border border-border-subtle bg-bg-elevated px-3 py-2 text-[0.76rem] text-fg-muted">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <span className="block text-fg-body">{scenarioPatchValidationStatus(validation)}</span>
          {validationProgress && <span className="block">{validationProgress}</span>}
          <span className="block font-mono break-all">{validation.trigger_url}</span>
        </div>
        <button
          type="button"
          onClick={onRefreshValidation}
          disabled={validationLoading}
          className="w-fit rounded-md border border-border bg-bg-alt px-2.5 py-1 text-[0.72rem] font-semibold text-fg transition-colors hover:border-accent disabled:cursor-default disabled:opacity-50"
        >
          {validationLoading ? "Refreshing..." : "Refresh"}
        </button>
      </div>
      {validationRunIDs.length > 0 && (
        <div className="mt-2 flex flex-wrap gap-1.5">
          {validationRunIDs.map((runID) => (
            <Link key={runID} to={benchRunPath(runID)} className="rounded border border-border-subtle bg-bg-alt px-2 py-0.5 font-mono text-[0.7rem] text-fg-muted hover:text-accent">
              {runID}
            </Link>
          ))}
        </div>
      )}
    </div>
  ) : null;

  if (!preview) {
    if (!validationPanel && !validationError) return null;
    return (
      <div className="mt-3 rounded-md border border-border-subtle bg-bg-alt/50 p-3">
        {validationPanel}
        {validationError && <div className="rounded bg-[var(--color-danger-badge-bg)] px-3 py-2 text-[0.76rem] text-[var(--color-danger-badge-fg)]">{validationError}</div>}
      </div>
    );
  }

  const diffHref = scenarioPatchPreviewDownloadHref(preview, API_BASE);

  return (
    <div className="mt-3 rounded-md border border-border-subtle bg-bg-alt/50 p-3">
      <div className="mb-2 flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <span className="block text-[0.8rem] font-semibold text-fg">{scenarioPatchPreviewStatus(preview)}</span>
          {preview.scenario_path && <span className="block font-mono text-[0.72rem] text-fg-muted break-all">{preview.scenario_path}</span>}
        </div>
        {diffHref && (
          <div className="flex flex-wrap gap-2">
            <a href={diffHref} download={scenarioPatchPreviewDiffFilename(preview)} className="w-fit rounded-md border border-border bg-bg-alt px-3 py-1.5 text-[0.78rem] font-semibold text-fg transition-colors hover:border-accent">
              Download diff
            </a>
            <button
              type="button"
              onClick={onValidate}
              disabled={validationLoading}
              className="w-fit rounded-md border border-border bg-bg-alt px-3 py-1.5 text-[0.78rem] font-semibold text-fg transition-colors hover:border-accent disabled:cursor-default disabled:opacity-50"
            >
              {validationLoading ? "Queuing..." : "Validate rerun"}
            </button>
          </div>
        )}
      </div>
      {validationPanel}
      {validationError && <div className="mb-2 rounded bg-[var(--color-danger-badge-bg)] px-3 py-2 text-[0.76rem] text-[var(--color-danger-badge-fg)]">{validationError}</div>}
      {preview.diff ? (
        <pre className="max-h-[360px] overflow-auto rounded border border-border-subtle bg-code-bg px-3 py-2 font-mono text-[0.72rem] leading-relaxed text-fg-muted whitespace-pre">
          {preview.diff}
        </pre>
      ) : (
        <p className="text-[0.8rem] text-fg-muted">No scenario YAML changes were produced.</p>
      )}
      {(preview.skipped_rules?.length ?? 0) > 0 && (
        <div className="mt-2 space-y-1">
          {preview.skipped_rules?.map((rule, index) => (
            <div key={index} className="font-mono text-[0.72rem] text-fg-muted break-words">
              skipped {rule.target || "rule"} {rule.pattern || ""}: {rule.reason || "not applied"}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

export function TranscriptTab({ transcript, loading, error }: { transcript: string | null; loading: boolean; error: string | null }) {
  if (error) return <p className="text-danger text-[0.82rem] py-6">Failed to load transcript: {error}</p>;
  if (!transcript && !loading) return <p className="text-fg-muted text-[0.82rem] py-6">No transcript available.</p>;
  return (
    <pre className={`bg-code-bg border border-border-subtle rounded-lg p-4 font-mono text-[0.78rem] leading-relaxed max-h-[500px] overflow-y-auto whitespace-pre-wrap break-words min-h-[100px] transition-opacity duration-200 ${loading ? "opacity-40 animate-pulse" : "opacity-100"}`}>
      {transcript ? highlightTranscript(transcript) : "\u00A0"}
    </pre>
  );
}

export function ToolCallsTab({ toolCalls, loading, error }: { toolCalls: ToolCall[] | null; loading: boolean; error: string | null }) {
  if (error) return <p className="text-danger text-[0.82rem] py-6">Failed to load tool calls: {error}</p>;
  if (!toolCalls && !loading) return <p className="text-fg-muted text-[0.82rem] py-6">No tool calls recorded.</p>;
  if (loading || !toolCalls || toolCalls.length === 0) {
    return (
      <div className={`min-h-[100px] rounded-lg bg-bg-alt/80 transition-opacity duration-200 ${loading ? "opacity-40 animate-pulse" : "opacity-100"}`}>
        {!loading && <p className="text-fg-muted text-[0.82rem] py-6 text-center">No tool calls recorded.</p>}
      </div>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-[0.8rem]">
        <thead>
          <tr className="border-b border-border-subtle text-fg-muted text-left">
            <th className="py-2 pr-3 font-semibold w-10">#</th>
            <th className="py-2 pr-3 font-semibold">Tool</th>
            <th className="py-2 pr-3 font-semibold">Arguments</th>
            <th className="py-2 font-semibold">Result</th>
          </tr>
        </thead>
        <tbody>
          {toolCalls.map((tc, i) => {
            const toolColor = tc.tool === "prescribe" ? "text-accent" : tc.tool === "report" ? "text-info" : "text-fg";
            return (
              <tr key={i} className="border-b border-border-subtle/50">
                <td className="py-2 pr-3 font-mono text-fg-muted">{i + 1}</td>
                <td className={`py-2 pr-3 font-mono font-semibold ${toolColor}`}>{tc.tool}</td>
                <td className="py-2 pr-3 font-mono text-fg-muted text-[0.75rem]">{truncate(typeof tc.args === "string" ? tc.args : JSON.stringify(tc.args), 80)}</td>
                <td className="py-2 font-mono text-fg-muted text-[0.75rem]">{truncate(typeof tc.result === "string" ? tc.result : JSON.stringify(tc.result), 80)}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

export function AutopsyTab({ autopsy, loading, error }: { autopsy: AutopsyReport | null; loading: boolean; error: string | null }) {
  if (loading) return <p className="text-fg-muted text-[0.82rem] py-6">Loading failure autopsy...</p>;
  if (error === "not-found" || (!autopsy && !loading && !error)) return <p className="text-fg-muted text-[0.82rem] py-6">No failure autopsy available.</p>;
  if (error) return <p className="text-danger text-[0.82rem] py-6">Failed to load failure autopsy: {error}</p>;
  if (!autopsy) return <p className="text-fg-muted text-[0.82rem] py-6">No failure autopsy available.</p>;

  const view = normalizeAutopsyReport(autopsy);
  const findings = view.findings;
  const metrics = view.metrics;

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className={`inline-block px-2 py-0.5 rounded text-[0.7rem] font-semibold uppercase tracking-wide ${view.outcome === "pass" ? "bg-accent-tint text-accent" : "bg-danger/15 text-danger"}`}>
              {view.outcome}
            </span>
            <span className={`inline-block px-2 py-0.5 rounded text-[0.7rem] font-semibold uppercase tracking-wide ${confidenceStyle(view.confidence)}`}>{view.confidence}</span>
            <span className="inline-block px-2 py-0.5 rounded bg-bg-alt/80 text-fg-muted text-[0.7rem] font-mono">{view.version}</span>
            <span className="text-fg font-semibold text-[1rem] break-words">{view.primaryLabel}</span>
          </div>
          <p className="text-fg-muted text-[0.82rem] mt-2 max-w-3xl break-words">{view.summary}</p>
        </div>
        <div className="flex flex-wrap gap-2 text-[0.78rem] flex-shrink-0 sm:justify-end">
          <span className="font-mono text-fg-muted bg-bg-alt/80 rounded-md px-2.5 py-1">wasted turns {view.waste.turns}</span>
          <span className="font-mono text-fg-muted bg-bg-alt/80 rounded-md px-2.5 py-1">wasted tokens {formatCompactTokens(view.waste.tokens)}</span>
          {view.waste.basis && <span className="font-mono text-fg-muted bg-bg-alt/80 rounded-md px-2.5 py-1">basis {view.waste.basis}</span>}
        </div>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
        <AutopsyMetric label="Turns" value={String(metrics.turns)} />
        <AutopsyMetric label="Tokens" value={formatCompactTokens(metrics.total_tokens)} />
        <AutopsyMetric label="Checks" value={`${metrics.checks_passed}/${metrics.checks_total}`} />
        <AutopsyMetric label="Cost" value={`$${metrics.estimated_cost_usd.toFixed(4)}`} />
        <AutopsyMetric label="Steps" value={String(metrics.total_steps)} />
        <AutopsyMetric label="Mutations" value={String(metrics.mutation_count)} />
        <AutopsyMetric label="Diagnosis" value={String(metrics.diagnosis_depth)} />
        <AutopsyMetric label="Prompt / Completion" value={`${formatCompactTokens(metrics.prompt_tokens)} / ${formatCompactTokens(metrics.completion_tokens)}`} />
      </div>

      <div>
        <h3 className="text-[0.9rem] font-semibold text-fg mb-3">Findings</h3>
        {findings.length > 0 ? (
          <div className="space-y-1.5">
            {findings.map((finding, i) => (
              <div key={`${finding.kind}-${i}`} className="flex flex-col gap-2 px-3 py-2.5 bg-bg-alt/80 rounded-md text-[0.8rem] sm:flex-row sm:items-start">
                <span className={`inline-block px-2 py-0.5 rounded text-[0.7rem] font-semibold uppercase tracking-wide flex-shrink-0 ${severityStyle(finding.severity)}`}>{finding.severity}</span>
                <div className="min-w-0 flex-1">
                  <div className="font-mono text-fg text-[0.78rem] break-words">{finding.kindLabel}</div>
                  <div className="text-fg-muted mt-0.5 break-words">{finding.message}</div>
                  {finding.evidenceText && <div className="font-mono text-fg-muted text-[0.72rem] mt-1 break-all">{finding.evidenceText}</div>}
                </div>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-fg-muted text-[0.82rem]">No deterministic failure pattern matched.</p>
        )}
      </div>
    </div>
  );
}

function AutopsyMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-bg-alt/80 rounded-md px-3 py-2 min-w-0">
      <div className="text-[0.65rem] font-semibold text-fg-muted uppercase tracking-wide">{label}</div>
      <div className="font-mono text-[0.78rem] font-semibold text-fg mt-0.5 break-words">{value}</div>
    </div>
  );
}

function severityStyle(severity: string): string {
  if (severity === "critical") return "bg-danger/15 text-danger";
  if (severity === "warning") return "bg-warning/15 text-warning";
  return "bg-accent-tint text-accent";
}

function confidenceStyle(confidence: string): string {
  if (confidence === "high") return "bg-accent-tint text-accent";
  if (confidence === "medium") return "bg-warning/15 text-warning";
  if (confidence === "low") return "bg-danger/15 text-danger";
  return "bg-bg-alt/80 text-fg-muted";
}

export function TimelineTab({ timeline, loading, error }: { timeline: TimelineData | null; loading: boolean; error: string | null }) {
  if (loading) return <p className="text-fg-muted text-[0.82rem] py-6">Loading timeline...</p>;
  if (error === "not-found" || (!timeline && !loading && !error)) return <p className="text-fg-muted text-[0.82rem] py-6">No timeline available.</p>;
  if (error) return <p className="text-danger text-[0.82rem] py-6">Failed to load timeline: {error}</p>;
  if (!timeline || timeline.steps.length === 0) return <p className="text-fg-muted text-[0.82rem] py-6">No timeline available.</p>;

  const summaryParts = PHASE_ORDER
    .filter((p) => timeline.phase_count[p] && timeline.phase_count[p] > 0)
    .map((p) => `${timeline.phase_count[p]} ${p}`);

  return (
    <div className="space-y-4">
      <p className="text-fg-muted text-[0.82rem]">
        {timeline.total_steps} step{timeline.total_steps !== 1 ? "s" : ""}
        {summaryParts.length > 0 && ": "}
        {summaryParts.join(" \u2192 ")}
      </p>
      <div className="space-y-1.5">
        {timeline.steps.map((step) => {
          const phaseStyle = PHASE_STYLES[step.phase] || PHASE_STYLES.explain;
          return (
            <div key={step.index} className="flex items-start gap-3 px-3 py-2 bg-bg-alt/80 rounded-md text-[0.8rem]">
              <span className="font-mono text-fg-muted text-[0.75rem] min-w-[1.5rem] text-right flex-shrink-0 pt-0.5">{step.index + 1}</span>
              <span className={`inline-block px-2 py-0.5 rounded text-[0.7rem] font-semibold uppercase tracking-wide flex-shrink-0 ${phaseStyle}`}>{step.phase}</span>
              <div className="min-w-0 flex-1">
                <span className="text-fg">{step.summary}</span>
                {step.command && <div className="font-mono text-fg-muted text-[0.72rem] mt-0.5 truncate">{step.command}</div>}
              </div>
              {step.exit_code !== 0 && <span className="text-[0.7rem] font-mono text-danger flex-shrink-0 pt-0.5">exit {step.exit_code}</span>}
            </div>
          );
        })}
      </div>
    </div>
  );
}

export function ScorecardTab({ scorecard, loading, error }: { scorecard: Scorecard | null; loading: boolean; error: string | null }) {
  if (loading) return <p className="text-fg-muted text-[0.82rem] py-6">Loading scorecard...</p>;
  if (error === "not-found" || (!scorecard && !loading && !error)) return <p className="text-fg-muted text-[0.82rem] py-6">No scorecard available.</p>;
  if (error) return <p className="text-danger text-[0.82rem] py-6">Failed to load scorecard: {error}</p>;
  if (!scorecard) return <p className="text-fg-muted text-[0.82rem] py-6">No scorecard available.</p>;

  const signals = scorecard.signals || {};

  return (
    <div className="space-y-6">
      <div className="flex items-baseline gap-4">
        <span className="font-mono text-accent font-bold" style={{ fontSize: "3rem", lineHeight: 1 }}>{scorecard.score}</span>
        <div>
          <span className="text-fg font-semibold text-[1rem]">{scorecard.band}</span>
          <p className="text-fg-muted text-[0.78rem] mt-0.5">{Object.keys(signals).length} signal{Object.keys(signals).length !== 1 ? "s" : ""} evaluated</p>
        </div>
      </div>
      {Object.keys(signals).length > 0 && (
        <div>
          <h3 className="text-[0.9rem] font-semibold text-fg mb-3">Signal Breakdown</h3>
          <table className="w-full text-[0.8rem]">
            <thead>
              <tr className="border-b border-border-subtle text-fg-muted text-left">
                <th className="py-2 pr-3 font-semibold">Signal</th>
                <th className="py-2 pr-3 font-semibold w-20">Count</th>
                <th className="py-2 font-semibold w-28">Status</th>
              </tr>
            </thead>
            <tbody>
              {Object.entries(signals).map(([name, count]) => (
                <tr key={name} className="border-b border-border-subtle/50">
                  <td className="py-2 pr-3 font-mono text-fg">{name}</td>
                  <td className="py-2 pr-3 font-mono text-fg-muted">{count}</td>
                  <td className="py-2">
                    <span className={`inline-block px-2 py-0.5 rounded text-[0.7rem] font-semibold ${count > 0 ? "bg-warning/15 text-warning" : "bg-accent-tint text-accent"}`}>
                      {count > 0 ? "detected" : "clear"}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
