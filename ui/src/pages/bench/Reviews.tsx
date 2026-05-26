import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router";
import { useBenchApi } from "../../hooks/useBenchApi";
import { usePageTitle } from "../../hooks/usePageTitle";
import { formatDateTime } from "../../lib/benchFormatters.mts";
import type { BenchRunRecord, BenchRunsResponse, BenchRunReviewSummary } from "../../lib/benchTypes.mts";
import { buildReviewQueue, reviewSeverityTone, reviewSummaryText } from "../../lib/reviewQueue.mts";
import { benchRunPath, BENCH_REVIEWS_PATH } from "../../lib/routes.mts";

const QUEUE_LIMIT = 200;

export function Reviews() {
  usePageTitle("Reviews", { canonicalPath: BENCH_REVIEWS_PATH });
  const { request } = useBenchApi();
  const navigate = useNavigate();
  const [runs, setRuns] = useState<BenchRunRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    request<BenchRunsResponse>(`/v1/bench/runs?limit=${QUEUE_LIMIT}`)
      .then((res) => {
        if (!cancelled) setRuns(res.runs ?? []);
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

  const queue = useMemo(() => buildReviewQueue(runs), [runs]);

  return (
    <div>
      <div className="mb-5">
        <h1 className="text-[1.35rem] font-bold text-fg tracking-tight">Reviews</h1>
        <p className="text-[0.82rem] text-fg-muted mt-0.5">
          Triage recent runs by human review state and evidence verdict.
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
            title="Needs Review"
            description="Recent runs without a saved human review."
            runs={queue.needsReview}
            navigate={navigate}
          />
          <QueueSection
            title="Unsafe Passes"
            description="Passed runs where human review marked unsafe behavior."
            runs={queue.unsafePasses}
            navigate={navigate}
          />
          <QueueSection
            title="Reviewed Failures"
            description="Failed runs that already have human review evidence."
            runs={queue.reviewedFailures}
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
  runs,
  navigate,
}: {
  title: string;
  description: string;
  runs: BenchRunRecord[];
  navigate: (path: string) => void;
}) {
  return (
    <section className="border border-border rounded-lg overflow-hidden bg-bg-elevated">
      <div className="flex items-start justify-between gap-4 px-4 py-3 border-b border-border-subtle bg-bg-alt/50">
        <div>
          <h2 className="text-[0.95rem] font-semibold text-fg">{title}</h2>
          <p className="text-[0.76rem] text-fg-muted mt-0.5">{description}</p>
        </div>
        <span className="font-mono text-[0.78rem] text-fg-muted">{runs.length}</span>
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
              {runs.slice(0, 12).map((run) => (
                <tr
                  key={run.id}
                  onClick={() => navigate(benchRunPath(run.id))}
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
                    <ReviewChip summary={run.review_summary} />
                  </td>
                  <td className="px-3 py-2.5 font-mono text-[0.76rem] text-fg-muted whitespace-nowrap">
                    {formatDateTime(run.created_at)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {runs.length > 12 && (
            <div className="px-4 py-2 text-[0.76rem] text-fg-muted border-t border-border-subtle">
              Showing first 12 of {runs.length}.
            </div>
          )}
        </div>
      )}
    </section>
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
