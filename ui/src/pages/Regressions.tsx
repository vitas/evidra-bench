import { useEffect, useState } from "react";
import { Link } from "react-router";
import { useApi } from "../hooks/useApi";
import { usePageTitle } from "../hooks/usePageTitle";
import { evidenceModeParam } from "../lib/catalogData.mts";

interface Regression {
  scenario_id: string;
  model: string;
  latest_run_id: string;
  latest_passed: boolean;
  prev_passed: number;
  prev_total: number;
  prev_rate: number;
  severity: string;
}

export function Regressions() {
  usePageTitle("Regressions");
  const { request } = useApi();
  const [regressions, setRegressions] = useState<Regression[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    request<Regression[]>(`/v1/bench/regressions${evidenceModeParam("?")}`)
      .then(setRegressions)
      .catch(() => setRegressions([]))
      .finally(() => setLoading(false));
  }, [request]);

  const critical = regressions.filter((r) => r.severity === "critical");
  const warnings = regressions.filter((r) => r.severity === "warning");

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20 text-fg-muted text-[0.85rem]">
        Scanning for regressions...
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-[1.4rem] font-bold text-fg tracking-tight">
          Regression Alerts
        </h1>
        <p className="text-[0.85rem] text-fg-muted mt-0.5">
          Scenarios where a model&apos;s latest run failed after previously passing
        </p>
      </div>

      {/* Status banner */}
      {regressions.length === 0 ? (
        <div className="bg-accent-subtle border border-accent rounded-[10px] p-6 flex items-center gap-4">
          <span className="text-[1.5rem]">{"\u2705"}</span>
          <div>
            <p className="text-fg font-semibold text-[0.95rem]">No regressions detected</p>
            <p className="text-fg-muted text-[0.82rem]">
              All models maintain or improve their pass rates across scenarios.
            </p>
          </div>
        </div>
      ) : (
        <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
          <MiniCard
            label="Total Regressions"
            value={String(regressions.length)}
            accent={critical.length > 0 ? "danger" : "warning"}
          />
          <MiniCard
            label="Critical"
            value={String(critical.length)}
            detail="Previously 80%+ pass rate"
            accent="danger"
          />
          <MiniCard
            label="Warnings"
            value={String(warnings.length)}
            detail="Previously <80% pass rate"
            accent="warning"
          />
        </div>
      )}

      {/* Regressions table */}
      {regressions.length > 0 && (
        <div className="bg-bg-elevated border border-border-subtle rounded-[10px] shadow-[var(--shadow-card)] overflow-hidden">
          <table className="w-full text-[0.82rem]">
            <thead>
              <tr className="border-b border-border bg-bg-alt">
                <th className="text-left text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-5 py-2.5">
                  Severity
                </th>
                <th className="text-left text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-4 py-2.5">
                  Scenario
                </th>
                <th className="text-left text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-4 py-2.5">
                  Model
                </th>
                <th className="text-center text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-4 py-2.5">
                  Previous
                </th>
                <th className="text-center text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-4 py-2.5">
                  Latest
                </th>
                <th className="text-right text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-5 py-2.5">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody>
              {regressions.map((reg) => (
                <tr
                  key={`${reg.scenario_id}-${reg.model}`}
                  className="border-b border-border-subtle last:border-0 hover:bg-accent-subtle transition-colors"
                >
                  <td className="px-5 py-3">
                    <span
                      className={`inline-flex items-center gap-1.5 font-mono text-[0.72rem] font-semibold px-2.5 py-1 rounded ${
                        reg.severity === "critical"
                          ? "bg-[var(--color-danger-badge-bg)] text-[var(--color-danger-badge-fg)]"
                          : "bg-warning-tint text-warning"
                      }`}
                    >
                      {reg.severity === "critical" ? "\u26D4" : "\u26A0"}{" "}
                      {reg.severity}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <Link
                      to={`/scenarios/${reg.scenario_id}`}
                      className="font-mono text-[0.82rem] text-accent hover:underline"
                    >
                      {reg.scenario_id}
                    </Link>
                  </td>
                  <td className="px-4 py-3 font-mono text-[0.82rem] text-fg">
                    {reg.model}
                  </td>
                  <td className="px-4 py-3 text-center">
                    <span className="font-mono text-[0.78rem] text-accent font-semibold">
                      {reg.prev_passed}/{reg.prev_total} ({reg.prev_rate.toFixed(0)}%)
                    </span>
                  </td>
                  <td className="px-4 py-3 text-center">
                    <span className="inline-block font-mono text-[0.72rem] font-semibold px-2 py-0.5 rounded bg-[var(--color-danger-badge-bg)] text-[var(--color-danger-badge-fg)]">
                      FAIL
                    </span>
                  </td>
                  <td className="px-4 py-3 text-right">
                    <Link
                      to={`/runs/${reg.latest_run_id}`}
                      className="text-accent text-[0.78rem] hover:underline"
                    >
                      View run &rarr;
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Explanation */}
      <div className="bg-bg-alt rounded-[10px] p-5">
        <h3 className="text-[0.85rem] font-semibold text-fg mb-2">How regressions are detected</h3>
        <ul className="text-[0.78rem] text-fg-muted space-y-1 list-disc list-inside">
          <li><strong className="text-fg">Critical</strong> — latest run FAILED, but previous pass rate was 80%+ (reliable scenario now broken)</li>
          <li><strong className="text-fg">Warning</strong> — latest run FAILED, but previous pass rate was below 80% (flaky scenario got worse)</li>
          <li>Only scenario/model pairs with at least one previous passing run are flagged</li>
          <li>Re-running the scenario clears the alert if it passes</li>
        </ul>
      </div>
    </div>
  );
}

function MiniCard({
  label,
  value,
  detail,
  accent,
}: {
  label: string;
  value: string;
  detail?: string;
  accent: string;
}) {
  return (
    <div className={`bg-bg-elevated border border-border-subtle rounded-lg p-3 shadow-[var(--shadow-card)] border-l-[3px] border-l-${accent}`}>
      <p className="text-[0.68rem] font-semibold uppercase tracking-wide text-fg-muted">
        {label}
      </p>
      <p className="font-mono text-[1.1rem] font-bold text-fg mt-0.5">{value}</p>
      {detail && <p className="text-[0.68rem] text-fg-muted mt-0.5">{detail}</p>}
    </div>
  );
}
