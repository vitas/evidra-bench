import { useEffect, useState } from "react";
import { useNavigate } from "react-router";
import { useBenchApi } from "../../hooks/useBenchApi";
import { usePageTitle } from "../../hooks/usePageTitle";
import { formatDateTime } from "../../lib/benchFormatters.mts";
import type { BenchRunsResponse, BenchRunReviewSummary } from "../../lib/benchTypes.mts";
import { reviewQueueApiPath, reviewSeverityTone, reviewSummaryText } from "../../lib/reviewQueue.mts";
import { benchRunPath, BENCH_REVIEWS_PATH } from "../../lib/routes.mts";

const QUEUE_LIMIT = 25;

interface ReviewQueueState {
  needsReview: BenchRunsResponse;
  unsafePasses: BenchRunsResponse;
  reviewedFailures: BenchRunsResponse;
  scenarioImprovements: BenchRunsResponse;
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
      request<BenchRunsResponse>(reviewQueueApiPath("needsReview", QUEUE_LIMIT)),
      request<BenchRunsResponse>(reviewQueueApiPath("unsafePasses", QUEUE_LIMIT)),
      request<BenchRunsResponse>(reviewQueueApiPath("reviewedFailures", QUEUE_LIMIT)),
      request<BenchRunsResponse>(reviewQueueApiPath("scenarioImprovements", QUEUE_LIMIT)),
    ])
      .then(([needsReview, unsafePasses, reviewedFailures, scenarioImprovements]) => {
        if (!cancelled) {
          setQueue({
            needsReview: normalizeRunsResponse(needsReview),
            unsafePasses: normalizeRunsResponse(unsafePasses),
            reviewedFailures: normalizeRunsResponse(reviewedFailures),
            scenarioImprovements: normalizeRunsResponse(scenarioImprovements),
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
          <QueueSection
            title="Scenario Improvements"
            description="Reviewed runs with suggested scenario rules ready for patch preview."
            response={queue.scenarioImprovements}
            navigate={navigate}
            openReviewTab
            showImprovementDetails
          />
          <QueueSection
            title="Needs Review"
            description="Runs without a caller-visible human review."
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

function emptyReviewQueueState(): ReviewQueueState {
  return {
    needsReview: emptyRunsResponse(),
    unsafePasses: emptyRunsResponse(),
    reviewedFailures: emptyRunsResponse(),
    scenarioImprovements: emptyRunsResponse(),
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
          {summary.primary_evidence_snippet && (
            <div className="font-mono text-[0.7rem] text-fg-muted truncate">
              {summary.primary_evidence_snippet}
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
