import { useEffect, useState } from "react";
import { useNavigate } from "react-router";
import { useBenchApi } from "../../hooks/useBenchApi";
import { usePageTitle } from "../../hooks/usePageTitle";
import { formatDateTime } from "../../lib/benchFormatters.mts";
import type {
  BenchRunsResponse,
  BenchReviewCandidate,
  BenchReviewCandidatesResponse,
  BenchRunReviewSummary,
  BenchScenarioImprovement,
  BenchScenarioImprovementsResponse,
} from "../../lib/benchTypes.mts";
import { reviewQueueApiPath, reviewSeverityTone, reviewSummaryText } from "../../lib/reviewQueue.mts";
import { benchRunPath, BENCH_REVIEWS_PATH } from "../../lib/routes.mts";

const QUEUE_LIMIT = 25;

interface ReviewQueueState {
  needsReview: BenchReviewCandidatesResponse;
  unsafePasses: BenchRunsResponse;
  reviewedFailures: BenchRunsResponse;
  scenarioImprovements: BenchScenarioImprovementsResponse;
}

export function Reviews() {
  usePageTitle("Reviews", { canonicalPath: BENCH_REVIEWS_PATH });
  const { request } = useBenchApi();
  const navigate = useNavigate();
  const [queue, setQueue] = useState<ReviewQueueState>(() => emptyReviewQueueState());
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    Promise.all([
      request<BenchReviewCandidatesResponse>(reviewQueueApiPath("needsReview", QUEUE_LIMIT)),
      request<BenchRunsResponse>(reviewQueueApiPath("unsafePasses", QUEUE_LIMIT)),
      request<BenchRunsResponse>(reviewQueueApiPath("reviewedFailures", QUEUE_LIMIT)),
      request<BenchScenarioImprovementsResponse>(reviewQueueApiPath("scenarioImprovements", QUEUE_LIMIT)),
    ])
      .then(([needsReview, unsafePasses, reviewedFailures, scenarioImprovements]) => {
        if (!cancelled) {
          setQueue({
            needsReview: normalizeReviewCandidatesResponse(needsReview),
            unsafePasses: normalizeRunsResponse(unsafePasses),
            reviewedFailures: normalizeRunsResponse(reviewedFailures),
            scenarioImprovements: normalizeScenarioImprovementsResponse(scenarioImprovements),
          });
        }
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "Failed to load review queue");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [request]);

  return (
    <div>
      <div className="mb-5">
        <h1 className="text-[1.35rem] font-bold text-fg tracking-tight">Reviews</h1>
        <p className="text-[0.82rem] text-fg-muted mt-0.5">
          Review runs by saved evidence verdict and missing final review.
        </p>
      </div>

      {error && (
        <div className="mb-4 px-4 py-3 rounded-md bg-[var(--color-danger-badge-bg)] text-[var(--color-danger-badge-fg)] text-[0.82rem]">
          {error}
        </div>
      )}

      {loading ? (
        <div className="flex items-center justify-center py-16 text-fg-muted text-[0.85rem]">
          Loading review queue...
        </div>
      ) : (
        <div className="space-y-5">
          <ScenarioImprovementSection
            title="Scenario Improvements"
            description="Reviewed runs with suggested scenario rules ready for patch preview."
            response={queue.scenarioImprovements}
            navigate={navigate}
          />
          <ReviewCandidateSection
            title="Needs Review"
            description="Unreviewed runs ranked by artifact coverage and likely review value."
            response={queue.needsReview}
            navigate={navigate}
          />
          <QueueSection
            title="Unsafe Passes"
            description="Passed runs where human review marked unsafe behavior."
            response={queue.unsafePasses}
            navigate={navigate}
          />
          <QueueSection
            title="Reviewed Failures"
            description="Failed runs that already have human review evidence."
            response={queue.reviewedFailures}
            navigate={navigate}
          />
        </div>
      )}
    </div>
  );
}

function QueueSection({
  title,
  description,
  response,
  navigate,
  openReviewTab,
  showImprovementDetails,
}: {
  title: string;
  description: string;
  response: BenchRunsResponse;
  navigate: (path: string) => void;
  openReviewTab?: boolean;
  showImprovementDetails?: boolean;
}) {
  const runs = response.runs ?? [];
  const total = response.total ?? runs.length;

  return (
    <section className="border border-border rounded-lg overflow-hidden bg-bg-elevated">
      <div className="flex items-start justify-between gap-4 px-4 py-3 border-b border-border-subtle bg-bg-alt/50">
        <div>
          <h2 className="text-[0.95rem] font-semibold text-fg">{title}</h2>
          <p className="text-[0.76rem] text-fg-muted mt-0.5">{description}</p>
        </div>
        <span className="font-mono text-[0.78rem] text-fg-muted">{total}</span>
      </div>
      {runs.length === 0 ? (
        <div className="px-4 py-6 text-[0.82rem] text-fg-muted">No runs in this queue.</div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full border-collapse">
            <thead>
              <tr className="border-b border-border-subtle">
                <th className="text-left text-[0.68rem] font-semibold text-fg-muted uppercase px-3 py-2">Run</th>
                <th className="text-left text-[0.68rem] font-semibold text-fg-muted uppercase px-3 py-2">Scenario</th>
                <th className="text-left text-[0.68rem] font-semibold text-fg-muted uppercase px-3 py-2">Status</th>
                <th className="text-left text-[0.68rem] font-semibold text-fg-muted uppercase px-3 py-2">Review</th>
                <th className="text-left text-[0.68rem] font-semibold text-fg-muted uppercase px-3 py-2">Date</th>
              </tr>
            </thead>
            <tbody>
              {runs.map((run) => (
                <tr
                  key={run.id}
                  onClick={() => navigate(openReviewTab ? `${benchRunPath(run.id)}?tab=review` : benchRunPath(run.id))}
                  className="border-b border-border-subtle cursor-pointer hover:bg-accent-subtle transition-colors"
                >
                  <td className="px-3 py-2.5 font-mono text-[0.76rem] text-fg-body max-w-[18rem] truncate">
                    {run.id}
                  </td>
                  <td className="px-3 py-2.5 font-mono text-[0.76rem] text-fg-body">{run.scenario_id}</td>
                  <td className="px-3 py-2.5">
                    <RunStatus passed={run.passed} />
                  </td>
                  <td className="px-3 py-2.5">
                    <ReviewCell summary={run.review_summary} showImprovementDetails={showImprovementDetails} />
                  </td>
                  <td className="px-3 py-2.5 font-mono text-[0.76rem] text-fg-muted whitespace-nowrap">
                    {formatDateTime(run.created_at)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {total > runs.length && (
            <div className="px-4 py-2 text-[0.76rem] text-fg-muted border-t border-border-subtle">
              Showing first {runs.length} of {total}.
            </div>
          )}
        </div>
      )}
    </section>
  );
}

function ReviewCandidateSection({
  title,
  description,
  response,
  navigate,
}: {
  title: string;
  description: string;
  response: BenchReviewCandidatesResponse;
  navigate: (path: string) => void;
}) {
  const candidates = response.candidates ?? [];
  const total = response.total ?? candidates.length;

  return (
    <section className="border border-border rounded-lg overflow-hidden bg-bg-elevated">
      <div className="flex items-start justify-between gap-4 px-4 py-3 border-b border-border-subtle bg-bg-alt/50">
        <div>
          <h2 className="text-[0.95rem] font-semibold text-fg">{title}</h2>
          <p className="text-[0.76rem] text-fg-muted mt-0.5">{description}</p>
        </div>
        <span className="font-mono text-[0.78rem] text-fg-muted">{total}</span>
      </div>
      {candidates.length === 0 ? (
        <div className="px-4 py-6 text-[0.82rem] text-fg-muted">No runs in this queue.</div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full border-collapse">
            <thead>
              <tr className="border-b border-border-subtle">
                <th className="text-left text-[0.68rem] font-semibold text-fg-muted uppercase px-3 py-2">Run</th>
                <th className="text-left text-[0.68rem] font-semibold text-fg-muted uppercase px-3 py-2">Scenario</th>
                <th className="text-left text-[0.68rem] font-semibold text-fg-muted uppercase px-3 py-2">Status</th>
                <th className="text-left text-[0.68rem] font-semibold text-fg-muted uppercase px-3 py-2">Reason</th>
                <th className="text-left text-[0.68rem] font-semibold text-fg-muted uppercase px-3 py-2">Artifacts</th>
                <th className="text-left text-[0.68rem] font-semibold text-fg-muted uppercase px-3 py-2">Date</th>
              </tr>
            </thead>
            <tbody>
              {candidates.map((item) => (
                <tr
                  key={item.run_id}
                  onClick={() => navigate(`${benchRunPath(item.run_id)}?tab=review&draft=1`)}
                  className="border-b border-border-subtle cursor-pointer hover:bg-accent-subtle transition-colors"
                >
                  <td className="px-3 py-2.5 font-mono text-[0.76rem] text-fg-body max-w-[18rem] truncate">
                    {item.run_id}
                  </td>
                  <td className="px-3 py-2.5 font-mono text-[0.76rem] text-fg-body">{item.scenario_id}</td>
                  <td className="px-3 py-2.5">
                    <RunStatus passed={item.passed} />
                  </td>
                  <td className="px-3 py-2.5">
                    <ReviewCandidateReason item={item} />
                  </td>
                  <td className="px-3 py-2.5">
                    <ArtifactCoverage coverage={item.artifact_coverage} />
                  </td>
                  <td className="px-3 py-2.5 font-mono text-[0.76rem] text-fg-muted whitespace-nowrap">
                    {formatDateTime(item.created_at)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {total > candidates.length && (
            <div className="px-4 py-2 text-[0.76rem] text-fg-muted border-t border-border-subtle">
              Showing first {candidates.length} of {total}.
            </div>
          )}
        </div>
      )}
    </section>
  );
}

function ScenarioImprovementSection({
  title,
  description,
  response,
  navigate,
}: {
  title: string;
  description: string;
  response: BenchScenarioImprovementsResponse;
  navigate: (path: string) => void;
}) {
  const improvements = response.improvements ?? [];
  const total = response.total ?? improvements.length;

  return (
    <section className="border border-border rounded-lg overflow-hidden bg-bg-elevated">
      <div className="flex items-start justify-between gap-4 px-4 py-3 border-b border-border-subtle bg-bg-alt/50">
        <div>
          <h2 className="text-[0.95rem] font-semibold text-fg">{title}</h2>
          <p className="text-[0.76rem] text-fg-muted mt-0.5">{description}</p>
        </div>
        <span className="font-mono text-[0.78rem] text-fg-muted">{total}</span>
      </div>
      {improvements.length === 0 ? (
        <div className="px-4 py-6 text-[0.82rem] text-fg-muted">No runs in this queue.</div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full border-collapse">
            <thead>
              <tr className="border-b border-border-subtle">
                <th className="text-left text-[0.68rem] font-semibold text-fg-muted uppercase px-3 py-2">Run</th>
                <th className="text-left text-[0.68rem] font-semibold text-fg-muted uppercase px-3 py-2">Scenario</th>
                <th className="text-left text-[0.68rem] font-semibold text-fg-muted uppercase px-3 py-2">Status</th>
                <th className="text-left text-[0.68rem] font-semibold text-fg-muted uppercase px-3 py-2">Review</th>
                <th className="text-left text-[0.68rem] font-semibold text-fg-muted uppercase px-3 py-2">Evidence</th>
                <th className="text-left text-[0.68rem] font-semibold text-fg-muted uppercase px-3 py-2">Date</th>
              </tr>
            </thead>
            <tbody>
              {improvements.map((item) => (
                <tr
                  key={item.run_id}
                  onClick={() => navigate(`${benchRunPath(item.run_id)}?tab=review`)}
                  className="border-b border-border-subtle cursor-pointer hover:bg-accent-subtle transition-colors"
                >
                  <td className="px-3 py-2.5 font-mono text-[0.76rem] text-fg-body max-w-[18rem] truncate">
                    {item.run_id}
                  </td>
                  <td className="px-3 py-2.5 font-mono text-[0.76rem] text-fg-body">{item.scenario_id}</td>
                  <td className="px-3 py-2.5">
                    <RunStatus passed={item.passed} />
                  </td>
                  <td className="px-3 py-2.5">
                    <ReviewCell summary={scenarioImprovementReviewSummary(item)} showImprovementDetails />
                  </td>
                  <td className="px-3 py-2.5">
                    <ImprovementEvidence item={item} />
                  </td>
                  <td className="px-3 py-2.5 font-mono text-[0.76rem] text-fg-muted whitespace-nowrap">
                    {formatDateTime(item.created_at)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {total > improvements.length && (
            <div className="px-4 py-2 text-[0.76rem] text-fg-muted border-t border-border-subtle">
              Showing first {improvements.length} of {total}.
            </div>
          )}
        </div>
      )}
    </section>
  );
}

function emptyReviewQueueState(): ReviewQueueState {
  return {
    needsReview: emptyReviewCandidatesResponse(),
    unsafePasses: emptyRunsResponse(),
    reviewedFailures: emptyRunsResponse(),
    scenarioImprovements: emptyScenarioImprovementsResponse(),
  };
}

function emptyRunsResponse(): BenchRunsResponse {
  return { runs: [], total: 0, limit: QUEUE_LIMIT, offset: 0 };
}

function normalizeRunsResponse(response: BenchRunsResponse): BenchRunsResponse {
  const runs = response.runs ?? [];
  return {
    ...response,
    runs,
    total: response.total ?? runs.length,
    limit: response.limit ?? QUEUE_LIMIT,
    offset: response.offset ?? 0,
  };
}

function emptyReviewCandidatesResponse(): BenchReviewCandidatesResponse {
  return { candidates: [], total: 0, limit: QUEUE_LIMIT, offset: 0 };
}

function normalizeReviewCandidatesResponse(response: BenchReviewCandidatesResponse): BenchReviewCandidatesResponse {
  const candidates = response.candidates ?? [];
  return {
    ...response,
    candidates,
    total: response.total ?? candidates.length,
    limit: response.limit ?? QUEUE_LIMIT,
    offset: response.offset ?? 0,
  };
}

function emptyScenarioImprovementsResponse(): BenchScenarioImprovementsResponse {
  return { improvements: [], total: 0, limit: QUEUE_LIMIT, offset: 0 };
}

function normalizeScenarioImprovementsResponse(
  response: BenchScenarioImprovementsResponse,
): BenchScenarioImprovementsResponse {
  const improvements = response.improvements ?? [];
  return {
    ...response,
    improvements,
    total: response.total ?? improvements.length,
    limit: response.limit ?? QUEUE_LIMIT,
    offset: response.offset ?? 0,
  };
}

function scenarioImprovementReviewSummary(item: BenchScenarioImprovement): BenchRunReviewSummary {
  return {
    verdict: item.verdict,
    primary_label: item.primary_label,
    visibility: item.visibility,
    label_count: 0,
    max_severity: item.max_severity,
    suggested_rule_count: item.suggested_rule_count,
    primary_evidence_snippet: item.primary_evidence_snippet,
  };
}

function ImprovementEvidence({ item }: { item: BenchScenarioImprovement }) {
  return (
    <div className="max-w-[24rem] space-y-1">
      {item.reviewer_note && (
        <div className="text-[0.76rem] text-fg-body leading-snug line-clamp-2">{item.reviewer_note}</div>
      )}
      {item.primary_evidence_snippet && (
        <div className="font-mono text-[0.7rem] text-fg-muted truncate">{item.primary_evidence_snippet}</div>
      )}
      {!item.reviewer_note && !item.primary_evidence_snippet && (
        <div className="text-[0.76rem] text-fg-muted">No evidence note.</div>
      )}
    </div>
  );
}

function ReviewCandidateReason({ item }: { item: BenchReviewCandidate }) {
  return (
    <div className="max-w-[22rem] space-y-1">
      <div className="text-[0.76rem] text-fg-body leading-snug">{item.reason}</div>
      <div className="flex flex-wrap items-center gap-1.5">
        <span className="font-mono text-[0.68rem] text-fg-muted">p{item.priority}</span>
        {item.signals.slice(0, 2).map((signal) => (
          <span key={signal} className="rounded bg-bg-alt px-1.5 py-0.5 font-mono text-[0.66rem] text-fg-muted">
            {signal}
          </span>
        ))}
      </div>
    </div>
  );
}

function ArtifactCoverage({ coverage }: { coverage: BenchReviewCandidate["artifact_coverage"] }) {
  const labels = [
    ["autopsy", coverage.failure_autopsy],
    ["timeline", coverage.timeline],
    ["tools", coverage.tool_calls],
    ["error", coverage.run_error],
    ["events", coverage.run_events],
  ] as const;

  return (
    <div className="flex max-w-[18rem] flex-wrap gap-1">
      {labels.map(([label, present]) => (
        <span
          key={label}
          className={`rounded px-1.5 py-0.5 font-mono text-[0.66rem] ${
            present ? "bg-accent-subtle text-fg-body" : "bg-bg-alt text-fg-muted opacity-60"
          }`}
        >
          {label}
        </span>
      ))}
    </div>
  );
}

function RunStatus({ passed }: { passed: boolean }) {
  return passed ? (
    <span className="bg-accent-tint text-accent font-mono text-[0.72rem] font-semibold px-2 py-0.5 rounded">
      PASS
    </span>
  ) : (
    <span className="bg-[var(--color-danger-badge-bg)] text-[var(--color-danger-badge-fg)] font-mono text-[0.72rem] font-semibold px-2 py-0.5 rounded">
      FAIL
    </span>
  );
}

function ReviewCell({
  summary,
  showImprovementDetails,
}: {
  summary?: BenchRunReviewSummary;
  showImprovementDetails?: boolean;
}) {
  return (
    <div className="max-w-[22rem]">
      <ReviewChip summary={summary} />
      {showImprovementDetails && summary && (
        <div className="mt-1 space-y-1">
          {(summary.suggested_rule_count ?? 0) > 0 && (
            <div className="font-mono text-[0.68rem] text-fg-muted">
              {summary.suggested_rule_count} suggested {summary.suggested_rule_count === 1 ? "rule" : "rules"}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function ReviewChip({ summary }: { summary?: BenchRunReviewSummary }) {
  const tone = reviewSeverityTone(summary);
  const toneClass = {
    none: "bg-bg-alt text-fg-muted border-border-subtle",
    info: "bg-accent-subtle text-fg-muted border-border-subtle",
    warning: "bg-warning-tint text-warning border-warning",
    error: "bg-[var(--color-danger-badge-bg)] text-[var(--color-danger-badge-fg)] border-[var(--color-danger-badge-fg)]/30",
    critical: "bg-[var(--color-danger-badge-bg)] text-[var(--color-danger-badge-fg)] border-[var(--color-danger-badge-fg)]/40",
  }[tone];

  return (
    <span className={`inline-flex max-w-[14rem] items-center truncate rounded border px-2 py-0.5 text-[0.72rem] font-medium ${toneClass}`}>
      {reviewSummaryText(summary)}
    </span>
  );
}
