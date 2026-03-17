import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router";
import { useApi } from "../hooks/useApi";
import { usePageTitle } from "../hooks/usePageTitle";

interface FailureInsights {
  scenario_id: string;
  total_runs: number;
  failed_runs: number;
  passed_runs: number;
  check_failures: { check_name: string; check_type: string; fail_count: number; fail_rate: number; message: string }[];
  command_patterns: { command: string; in_pass_runs: number; in_fail_runs: number; indicator: string }[];
  model_breakdown: { model: string; runs: number; passed: number; failed: number; rate: number }[];
  behavior_metrics: {
    pass_avg_turns: number; fail_avg_turns: number;
    pass_avg_duration: number; fail_avg_duration: number;
    pass_avg_tokens: number; fail_avg_tokens: number;
    pass_avg_cost: number; fail_avg_cost: number;
  };
}

interface Scenario { id: string; title: string }
interface ScenariosResponse { items: Scenario[] }

function fmt(n: number): string { return n.toFixed(1); }
function fmtDur(s: number): string { return s < 60 ? `${s.toFixed(1)}s` : `${Math.floor(s/60)}m ${Math.round(s%60)}s`; }
function fmtCost(n: number): string { return n < 0.01 ? `$${n.toFixed(3)}` : `$${n.toFixed(2)}`; }
function fmtTokens(n: number): string { return n >= 1000 ? `${(n/1000).toFixed(1)}k` : String(Math.round(n)); }

export function Insights() {
  usePageTitle("Failure Analysis");
  const { request } = useApi();
  const [scenarios, setScenarios] = useState<Scenario[]>([]);
  const [selected, setSelected] = useState("");
  const [insights, setInsights] = useState<FailureInsights | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    Promise.all([
      request<ScenariosResponse>("/v1/bench/scenarios"),
      request<{ by_scenario: { scenario_id: string; runs: number; passed: number }[] }>("/v1/bench/stats"),
    ])
      .then(([scenariosRes, stats]) => {
        const items = scenariosRes.items ?? [];
        setScenarios(items);
        // Auto-select the hardest scenario with enough data
        const ranked = [...(stats.by_scenario ?? [])]
          .filter((s) => s.runs >= 3 && s.passed < s.runs)
          .sort((a, b) => a.passed / a.runs - b.passed / b.runs);
        if (ranked.length > 0) {
          setSelected(ranked[0].scenario_id);
        } else if (items.length > 0) {
          setSelected(items[0].id);
        }
      })
      .catch(() => {});
  }, [request]);

  useEffect(() => {
    if (!selected) return;
    setLoading(true);
    request<FailureInsights>(`/v1/bench/insights?scenario=${encodeURIComponent(selected)}`)
      .then(setInsights)
      .catch(() => setInsights(null))
      .finally(() => setLoading(false));
  }, [selected, request]);

  const failSignals = useMemo(
    () => (insights?.command_patterns ?? []).filter((c) => c.indicator === "fail_signal"),
    [insights],
  );
  const passSignals = useMemo(
    () => (insights?.command_patterns ?? []).filter((c) => c.indicator === "pass_signal"),
    [insights],
  );

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-[1.4rem] font-bold text-fg tracking-tight">Failure Analysis</h1>
        <p className="text-[0.85rem] text-fg-muted mt-0.5">
          Extract patterns from failed runs — what goes wrong and why
        </p>
      </div>

      {/* Scenario picker */}
      <div className="flex items-center gap-3">
        <span className="text-[0.82rem] text-fg-muted font-medium">Scenario:</span>
        <select
          value={selected}
          onChange={(e) => setSelected(e.target.value)}
          className="h-9 px-3 text-[0.83rem] text-fg bg-bg-elevated border border-border-subtle rounded-lg cursor-pointer flex-1 max-w-md"
        >
          <option value="">Select scenario...</option>
          {scenarios.map((s) => (
            <option key={s.id} value={s.id}>{s.id} — {s.title}</option>
          ))}
        </select>
        {selected && (
          <Link to={`/scenarios/${selected}`} className="text-accent text-[0.78rem] hover:underline">
            View scenario &rarr;
          </Link>
        )}
      </div>

      {loading && (
        <div className="text-fg-muted text-[0.85rem] py-8 text-center animate-pulse">
          Analyzing failure patterns...
        </div>
      )}

      {insights && !loading && (
        <>
          {/* Summary */}
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
            <MiniCard label="Total Runs" value={String(insights.total_runs)} />
            <MiniCard label="Failed" value={String(insights.failed_runs)} accent="danger" />
            <MiniCard label="Passed" value={String(insights.passed_runs)} accent="accent" />
            <MiniCard
              label="Fail Rate"
              value={`${insights.total_runs > 0 ? ((insights.failed_runs / insights.total_runs) * 100).toFixed(0) : 0}%`}
              accent={insights.failed_runs > insights.passed_runs ? "danger" : "accent"}
            />
          </div>

          {/* Check failures — the "what failed" */}
          {(insights.check_failures ?? []).length > 0 && (
            <div className="bg-bg-elevated border border-border-subtle rounded-[10px] shadow-[var(--shadow-card)] overflow-hidden">
              <div className="px-5 pt-4 pb-2">
                <h2 className="text-[0.95rem] font-semibold text-fg">What Failed</h2>
                <p className="text-[0.72rem] text-fg-muted">Check failures across {insights.failed_runs} failed runs</p>
              </div>
              <table className="w-full text-[0.82rem]">
                <thead>
                  <tr className="border-t border-b border-border bg-bg-alt">
                    <th className="text-left text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-5 py-2">Check</th>
                    <th className="text-left text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-4 py-2">Type</th>
                    <th className="text-center text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-4 py-2">Count</th>
                    <th className="text-center text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-4 py-2">% of Failures</th>
                    <th className="text-left text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-5 py-2">Message</th>
                  </tr>
                </thead>
                <tbody>
                  {(insights.check_failures ?? []).map((cf) => (
                    <tr key={`${cf.check_type}/${cf.check_name}`} className="border-b border-border-subtle last:border-0">
                      <td className="px-5 py-2.5 font-mono text-[0.78rem] text-fg">{cf.check_name}</td>
                      <td className="px-4 py-2.5 text-fg-muted text-[0.76rem]">{cf.check_type}</td>
                      <td className="px-4 py-2.5 text-center font-mono text-[0.78rem] text-danger font-semibold">{cf.fail_count}</td>
                      <td className="px-4 py-2.5 text-center">
                        <div className="flex items-center justify-center gap-2">
                          <div className="w-16 h-1.5 rounded-full bg-bg-alt overflow-hidden">
                            <div className="h-full rounded-full bg-danger" style={{ width: `${cf.fail_rate}%` }} />
                          </div>
                          <span className="font-mono text-[0.72rem] text-danger">{cf.fail_rate.toFixed(0)}%</span>
                        </div>
                      </td>
                      <td className="px-5 py-2.5 text-fg-muted text-[0.74rem]">{cf.message}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {/* Command patterns — the "why it failed" */}
          {(failSignals.length > 0 || passSignals.length > 0) && (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {failSignals.length > 0 && (
                <div className="bg-bg-elevated border border-border-subtle rounded-[10px] shadow-[var(--shadow-card)] overflow-hidden">
                  <div className="px-5 pt-4 pb-2">
                    <h2 className="text-[0.95rem] font-semibold text-danger">Failure Signals</h2>
                    <p className="text-[0.72rem] text-fg-muted">Commands only seen in failed runs</p>
                  </div>
                  <div className="px-5 pb-4 space-y-2">
                    {failSignals.map((cmd) => (
                      <div key={cmd.command} className="flex items-center gap-3 bg-danger-tint rounded-md px-3 py-2">
                        <span className="text-danger text-[0.78rem]">{"\u26D4"}</span>
                        <code className="font-mono text-[0.76rem] text-fg flex-1">{cmd.command}</code>
                        <span className="font-mono text-[0.7rem] text-danger">{cmd.in_fail_runs} runs</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
              {passSignals.length > 0 && (
                <div className="bg-bg-elevated border border-border-subtle rounded-[10px] shadow-[var(--shadow-card)] overflow-hidden">
                  <div className="px-5 pt-4 pb-2">
                    <h2 className="text-[0.95rem] font-semibold text-accent">Success Signals</h2>
                    <p className="text-[0.72rem] text-fg-muted">Commands only seen in passing runs</p>
                  </div>
                  <div className="px-5 pb-4 space-y-2">
                    {passSignals.map((cmd) => (
                      <div key={cmd.command} className="flex items-center gap-3 bg-accent-subtle rounded-md px-3 py-2">
                        <span className="text-accent text-[0.78rem]">{"\u2705"}</span>
                        <code className="font-mono text-[0.76rem] text-fg flex-1">{cmd.command}</code>
                        <span className="font-mono text-[0.7rem] text-accent">{cmd.in_pass_runs} runs</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}

          {/* Behavior comparison */}
          <div className="bg-bg-elevated border border-border-subtle rounded-[10px] shadow-[var(--shadow-card)] overflow-hidden">
            <div className="px-5 pt-4 pb-2">
              <h2 className="text-[0.95rem] font-semibold text-fg">Behavior Comparison</h2>
              <p className="text-[0.72rem] text-fg-muted">How pass and fail runs differ in execution metrics</p>
            </div>
            <table className="w-full text-[0.82rem]">
              <thead>
                <tr className="border-t border-b border-border bg-bg-alt">
                  <th className="text-left text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-5 py-2">Metric</th>
                  <th className="text-center text-[0.7rem] font-semibold uppercase tracking-wide text-accent px-4 py-2">Pass Avg</th>
                  <th className="text-center text-[0.7rem] font-semibold uppercase tracking-wide text-danger px-4 py-2">Fail Avg</th>
                  <th className="text-center text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-5 py-2">Insight</th>
                </tr>
              </thead>
              <tbody>
                <BehaviorRow label="Turns" pass={fmt(insights.behavior_metrics.pass_avg_turns)} fail={fmt(insights.behavior_metrics.fail_avg_turns)} diff={insights.behavior_metrics.fail_avg_turns - insights.behavior_metrics.pass_avg_turns} unit="" moreIsBad />
                <BehaviorRow label="Duration" pass={fmtDur(insights.behavior_metrics.pass_avg_duration)} fail={fmtDur(insights.behavior_metrics.fail_avg_duration)} diff={insights.behavior_metrics.fail_avg_duration - insights.behavior_metrics.pass_avg_duration} unit="s" moreIsBad />
                <BehaviorRow label="Tokens" pass={fmtTokens(insights.behavior_metrics.pass_avg_tokens)} fail={fmtTokens(insights.behavior_metrics.fail_avg_tokens)} diff={insights.behavior_metrics.fail_avg_tokens - insights.behavior_metrics.pass_avg_tokens} unit="" moreIsBad />
                <BehaviorRow label="Cost" pass={fmtCost(insights.behavior_metrics.pass_avg_cost)} fail={fmtCost(insights.behavior_metrics.fail_avg_cost)} diff={insights.behavior_metrics.fail_avg_cost - insights.behavior_metrics.pass_avg_cost} unit="" moreIsBad />
              </tbody>
            </table>
          </div>

          {/* Model breakdown */}
          {(insights.model_breakdown ?? []).length > 0 && (
            <div className="bg-bg-elevated border border-border-subtle rounded-[10px] shadow-[var(--shadow-card)] overflow-hidden">
              <div className="px-5 pt-4 pb-2">
                <h2 className="text-[0.95rem] font-semibold text-fg">Model Breakdown</h2>
                <p className="text-[0.72rem] text-fg-muted">Which models pass and fail this scenario</p>
              </div>
              <div className="px-5 pb-4 space-y-2">
                {(insights.model_breakdown ?? []).map((m) => (
                  <div key={m.model} className="flex items-center gap-3">
                    <span className="font-mono text-[0.78rem] text-fg font-semibold w-44 truncate">{m.model}</span>
                    <div className="flex-1 h-2 rounded-full bg-bg-alt overflow-hidden">
                      <div
                        className={`h-full rounded-full ${m.rate >= 70 ? "bg-accent" : m.rate >= 40 ? "bg-warning" : "bg-danger"}`}
                        style={{ width: `${m.rate}%` }}
                      />
                    </div>
                    <span className={`font-mono text-[0.76rem] font-semibold w-12 text-right ${m.rate >= 70 ? "text-accent" : m.rate >= 40 ? "text-warning" : "text-danger"}`}>
                      {m.rate.toFixed(0)}%
                    </span>
                    <span className="font-mono text-[0.7rem] text-fg-muted w-16 text-right">{m.passed}/{m.runs}</span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}

function MiniCard({ label, value, accent }: { label: string; value: string; accent?: string }) {
  return (
    <div className={`bg-bg-elevated border border-border-subtle rounded-lg p-3 shadow-[var(--shadow-card)] ${accent ? `border-l-[3px] border-l-${accent}` : ""}`}>
      <p className="text-[0.68rem] font-semibold uppercase tracking-wide text-fg-muted">{label}</p>
      <p className="font-mono text-[1.1rem] font-bold text-fg mt-0.5">{value}</p>
    </div>
  );
}

function BehaviorRow({ label, pass, fail, diff, moreIsBad }: { label: string; pass: string; fail: string; diff: number; unit?: string; moreIsBad: boolean }) {
  const absDiff = Math.abs(diff);
  const significant = absDiff > 0.1;
  const worse = moreIsBad ? diff > 0 : diff < 0;
  return (
    <tr className="border-b border-border-subtle last:border-0">
      <td className="px-5 py-2.5 font-medium text-fg">{label}</td>
      <td className="px-4 py-2.5 text-center font-mono text-[0.78rem] text-accent">{pass}</td>
      <td className="px-4 py-2.5 text-center font-mono text-[0.78rem] text-danger">{fail}</td>
      <td className="px-5 py-2.5 text-center text-[0.76rem]">
        {significant ? (
          <span className={worse ? "text-danger" : "text-accent"}>
            {worse ? "Fail runs use more" : "Pass runs use more"}
          </span>
        ) : (
          <span className="text-fg-muted">Similar</span>
        )}
      </td>
    </tr>
  );
}
