import { useEffect, useMemo, useState } from "react";
import { useBenchApi as useApi } from "../../hooks/useBenchApi";
import { usePageTitle } from "../../hooks/usePageTitle";
import { evidenceModeParam } from "../../lib/catalogData.mts";
import { useEvidenceMode } from "../../hooks/useEvidenceMode";

/* ── Types ── */

interface Run {
  id: string;
  scenario_id: string;
  model: string;
  passed: boolean;
  duration_seconds: number;
  prompt_tokens: number;
  completion_tokens: number;
  estimated_cost_usd: number;
  checks_passed: number;
  checks_total: number;
  metadata_json: string;
}

interface RunsResponse {
  runs: Run[];
  total: number;
}

interface SkillGroup {
  model: string;
  without: { runs: number; passed: number; rate: number; avgCost: number; avgDuration: number };
  with: { runs: number; passed: number; rate: number; avgCost: number; avgDuration: number };
  delta: number; // rate difference (with - without)
}

interface ScenarioPair {
  scenario: string;
  withoutPassed: boolean | null;
  withPassed: boolean | null;
  change: "improved" | "regressed" | "same" | "no_data";
}

/* ── Helpers ── */

function hasSkill(metadataJson: string | null | undefined): boolean {
  return (metadataJson ?? "").includes("skill_version");
}

function formatPct(n: number): string {
  return `${n.toFixed(1)}%`;
}

function formatCost(usd: number): string {
  if (usd === 0) return "$0.00";
  if (usd < 0.01) return `$${usd.toFixed(3)}`;
  return `$${usd.toFixed(2)}`;
}

function formatDuration(s: number): string {
  if (s < 60) return `${s.toFixed(1)}s`;
  return `${Math.floor(s / 60)}m ${Math.round(s % 60)}s`;
}

function deltaColor(delta: number): string {
  if (delta > 5) return "text-accent";
  if (delta < -5) return "text-danger";
  return "text-fg-muted";
}

function deltaArrow(delta: number): string {
  if (delta > 5) return "\u2191";
  if (delta < -5) return "\u2193";
  return "\u2194";
}

function computeGroup(runs: Run[], withSkill: boolean) {
  const filtered = runs.filter((r) => hasSkill(r.metadata_json) === withSkill);
  const passed = filtered.filter((r) => r.passed).length;
  return {
    runs: filtered.length,
    passed,
    rate: filtered.length > 0 ? (passed / filtered.length) * 100 : 0,
    avgCost:
      filtered.length > 0
        ? filtered.reduce((s, r) => s + r.estimated_cost_usd, 0) / filtered.length
        : 0,
    avgDuration:
      filtered.length > 0
        ? filtered.reduce((s, r) => s + r.duration_seconds, 0) / filtered.length
        : 0,
  };
}

/* ── Component ── */

export function SkillImpact() {
  usePageTitle("Skill Impact");
  const { request } = useApi();
  const { mode } = useEvidenceMode();
  const [runs, setRuns] = useState<Run[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    request<RunsResponse>(`/v1/bench/runs?limit=1000${evidenceModeParam("&", mode)}`)
      .then((res) => setRuns(res.runs ?? []))
      .catch(() => setRuns([]))
      .finally(() => setLoading(false));
  }, [request, mode]);

  const hasAnySkillRuns = useMemo(
    () => runs.some((r) => hasSkill(r.metadata_json)),
    [runs],
  );

  // Group by model
  const modelGroups = useMemo(() => {
    const modelMap = new Map<string, Run[]>();
    for (const r of runs) {
      const arr = modelMap.get(r.model) ?? [];
      arr.push(r);
      modelMap.set(r.model, arr);
    }

    const groups: SkillGroup[] = [];
    for (const [model, modelRuns] of modelMap) {
      const without = computeGroup(modelRuns, false);
      const withG = computeGroup(modelRuns, true);
      if (without.runs === 0 && withG.runs === 0) continue;
      groups.push({
        model,
        without,
        with: withG,
        delta: withG.rate - without.rate,
      });
    }

    groups.sort((a, b) => b.delta - a.delta);
    return groups;
  }, [runs]);

  // Per-scenario pairs for models that have both
  const scenarioPairs = useMemo(() => {
    const pairs: ScenarioPair[] = [];
    const scenarioMap = new Map<string, { with: boolean[]; without: boolean[] }>();

    for (const r of runs) {
      const entry = scenarioMap.get(r.scenario_id) ?? { with: [], without: [] };
      if (hasSkill(r.metadata_json)) {
        entry.with.push(r.passed);
      } else {
        entry.without.push(r.passed);
      }
      scenarioMap.set(r.scenario_id, entry);
    }

    for (const [scenario, data] of scenarioMap) {
      if (data.with.length === 0 && data.without.length === 0) continue;
      const withRate = data.with.length > 0 ? data.with.filter(Boolean).length / data.with.length : null;
      const withoutRate = data.without.length > 0 ? data.without.filter(Boolean).length / data.without.length : null;

      let change: ScenarioPair["change"] = "no_data";
      if (withRate !== null && withoutRate !== null) {
        if (withRate > withoutRate + 0.05) change = "improved";
        else if (withRate < withoutRate - 0.05) change = "regressed";
        else change = "same";
      }

      pairs.push({
        scenario,
        withoutPassed: withoutRate !== null ? withoutRate >= 0.5 : null,
        withPassed: withRate !== null ? withRate >= 0.5 : null,
        change,
      });
    }

    pairs.sort((a, b) => {
      const order = { improved: 0, regressed: 1, same: 2, no_data: 3 };
      return order[a.change] - order[b.change];
    });
    return pairs;
  }, [runs]);

  const improved = scenarioPairs.filter((p) => p.change === "improved").length;
  const regressed = scenarioPairs.filter((p) => p.change === "regressed").length;
  const same = scenarioPairs.filter((p) => p.change === "same").length;

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20 text-fg-muted text-[0.85rem]">
        Loading skill impact data...
      </div>
    );
  }

  if (!hasAnySkillRuns) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-[1.4rem] font-bold text-fg tracking-tight">
            Skill Impact
          </h1>
          <p className="text-[0.85rem] text-fg-muted mt-0.5">
            How a system prompt skill affects agent reliability
          </p>
        </div>
        <div className="glass-card p-8 text-center">
          <p className="text-fg-muted text-[0.9rem] mb-2">No skill-enabled runs yet</p>
          <p className="text-fg-muted text-[0.78rem]">
            Run benchmarks with <code className="font-mono bg-bg-alt/80 px-1.5 py-0.5 rounded text-accent">--system-prompt-file</code> pointing
            to a skill prompt to see the impact comparison.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-[1.4rem] font-bold text-fg tracking-tight">
          Skill Impact
        </h1>
        <p className="text-[0.85rem] text-fg-muted mt-0.5">
          How a system prompt skill affects agent reliability
        </p>
      </div>

      {/* Summary cards */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <MiniCard
          label="Skill Runs"
          value={String(runs.filter((r) => hasSkill(r.metadata_json)).length)}
          detail={`of ${runs.length} total`}
        />
        <MiniCard
          label="Scenarios Improved"
          value={String(improved)}
          detail={`${regressed} regressed, ${same} same`}
          accent={improved > regressed ? "accent" : "warning"}
        />
        <MiniCard
          label="Without Skill"
          value={formatPct(
            (() => {
              const without = runs.filter((r) => !hasSkill(r.metadata_json));
              return without.length > 0
                ? (without.filter((r) => r.passed).length / without.length) * 100
                : 0;
            })(),
          )}
          detail="pass rate"
        />
        <MiniCard
          label="With Skill"
          value={formatPct(
            (() => {
              const withS = runs.filter((r) => hasSkill(r.metadata_json));
              return withS.length > 0
                ? (withS.filter((r) => r.passed).length / withS.length) * 100
                : 0;
            })(),
          )}
          detail="pass rate"
          accent="accent"
        />
      </div>

      {/* Model comparison table */}
      <div className="glass-card overflow-hidden">
        <div className="px-5 pt-4 pb-2">
          <h2 className="text-[0.95rem] font-semibold text-fg">
            Pass Rate by Model
          </h2>
          <p className="text-[0.72rem] text-fg-muted">
            Comparing runs with and without the selected skill prompt
          </p>
        </div>
        <table className="w-full text-[0.82rem]">
          <thead>
            <tr className="border-b border-t border-border bg-bg-alt/80">
              <th className="text-left text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-5 py-2">
                Model
              </th>
              <th className="text-center text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-4 py-2">
                Without Skill
              </th>
              <th className="text-center text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-4 py-2">
                With Skill
              </th>
              <th className="text-center text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-4 py-2">
                Delta
              </th>
              <th className="text-center text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-4 py-2">
                Cost Impact
              </th>
            </tr>
          </thead>
          <tbody>
            {modelGroups.map((g) => (
              <tr
                key={g.model}
                className="border-b border-border-subtle last:border-0 hover:bg-accent-subtle transition-colors"
              >
                <td className="px-5 py-3">
                  <span className="font-mono text-[0.85rem] font-semibold text-fg">
                    {g.model}
                  </span>
                </td>
                <td className="px-4 py-3 text-center">
                  {g.without.runs > 0 ? (
                    <div>
                      <span className="font-mono text-[0.82rem] font-semibold text-fg">
                        {formatPct(g.without.rate)}
                      </span>
                      <br />
                      <span className="text-[0.68rem] text-fg-muted">
                        {g.without.passed}/{g.without.runs} &middot; {formatDuration(g.without.avgDuration)}
                      </span>
                    </div>
                  ) : (
                    <span className="text-fg-muted">{"\u2014"}</span>
                  )}
                </td>
                <td className="px-4 py-3 text-center">
                  {g.with.runs > 0 ? (
                    <div>
                      <span className="font-mono text-[0.82rem] font-semibold text-fg">
                        {formatPct(g.with.rate)}
                      </span>
                      <br />
                      <span className="text-[0.68rem] text-fg-muted">
                        {g.with.passed}/{g.with.runs} &middot; {formatDuration(g.with.avgDuration)}
                      </span>
                    </div>
                  ) : (
                    <span className="text-fg-muted">{"\u2014"}</span>
                  )}
                </td>
                <td className="px-4 py-3 text-center">
                  {g.without.runs > 0 && g.with.runs > 0 ? (
                    <span className={`font-mono text-[0.85rem] font-bold ${deltaColor(g.delta)}`}>
                      {deltaArrow(g.delta)} {g.delta > 0 ? "+" : ""}{g.delta.toFixed(1)}%
                    </span>
                  ) : (
                    <span className="text-fg-muted text-[0.78rem]">need data</span>
                  )}
                </td>
                <td className="px-4 py-3 text-center font-mono text-[0.76rem] text-fg-muted">
                  {g.without.runs > 0 && g.with.runs > 0
                    ? `${formatCost(g.without.avgCost)} → ${formatCost(g.with.avgCost)}`
                    : "\u2014"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Scenario-level impact */}
      <div className="glass-card overflow-hidden">
        <div className="px-5 pt-4 pb-2">
          <h2 className="text-[0.95rem] font-semibold text-fg">
            Scenario Impact
          </h2>
          <p className="text-[0.72rem] text-fg-muted">
            How skill affects pass/fail per scenario (across all models with both runs)
          </p>
        </div>
        <table className="w-full text-[0.82rem]">
          <thead>
            <tr className="border-b border-t border-border bg-bg-alt/80">
              <th className="text-left text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-5 py-2">
                Scenario
              </th>
              <th className="text-center text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-4 py-2">
                Without
              </th>
              <th className="text-center text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-4 py-2">
                With
              </th>
              <th className="text-center text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-4 py-2">
                Impact
              </th>
            </tr>
          </thead>
          <tbody>
            {scenarioPairs
              .filter((p) => p.change !== "no_data")
              .map((p) => (
                <tr
                  key={p.scenario}
                  className="border-b border-border-subtle last:border-0 hover:bg-accent-subtle transition-colors"
                >
                  <td className="px-5 py-2.5 font-mono text-[0.78rem] text-fg">
                    {p.scenario}
                  </td>
                  <td className="px-4 py-2.5 text-center">
                    {p.withoutPassed !== null ? (
                      <span
                        className={`inline-block font-mono text-[0.7rem] font-semibold px-2 py-0.5 rounded ${
                          p.withoutPassed
                            ? "bg-accent-tint text-accent"
                            : "bg-[var(--color-danger-badge-bg)] text-[var(--color-danger-badge-fg)]"
                        }`}
                      >
                        {p.withoutPassed ? "PASS" : "FAIL"}
                      </span>
                    ) : (
                      "\u2014"
                    )}
                  </td>
                  <td className="px-4 py-2.5 text-center">
                    {p.withPassed !== null ? (
                      <span
                        className={`inline-block font-mono text-[0.7rem] font-semibold px-2 py-0.5 rounded ${
                          p.withPassed
                            ? "bg-accent-tint text-accent"
                            : "bg-[var(--color-danger-badge-bg)] text-[var(--color-danger-badge-fg)]"
                        }`}
                      >
                        {p.withPassed ? "PASS" : "FAIL"}
                      </span>
                    ) : (
                      "\u2014"
                    )}
                  </td>
                  <td className="px-4 py-2.5 text-center">
                    <span
                      className={`font-semibold text-[0.78rem] ${
                        p.change === "improved"
                          ? "text-accent"
                          : p.change === "regressed"
                            ? "text-danger"
                            : "text-fg-muted"
                      }`}
                    >
                      {p.change === "improved"
                        ? "\u2191 improved"
                        : p.change === "regressed"
                          ? "\u2193 regressed"
                          : "\u2194 same"}
                    </span>
                  </td>
                </tr>
              ))}
          </tbody>
        </table>
      </div>

      {/* CTA */}
      <div className="bg-accent-subtle border border-accent rounded-[10px] p-5 text-center">
        <p className="text-fg text-[0.9rem] font-semibold mb-1">
          Want more data?
        </p>
        <p className="text-fg-muted text-[0.8rem]">
          Run benchmarks with a skill prompt to build a complete comparison.
          Use <code className="font-mono bg-bg-alt/80 px-1.5 py-0.5 rounded text-accent">--system-prompt-file</code> with
          the contract prompt to enable skill-mode runs.
        </p>
      </div>
    </div>
  );
}

/* ── Sub-components ── */

function MiniCard({
  label,
  value,
  detail,
  accent,
}: {
  label: string;
  value: string;
  detail?: string;
  accent?: string;
}) {
  return (
    <div className={`glass-card p-3 ${accent ? `border-l-[3px] border-l-${accent}` : ""}`}>
      <p className="text-[0.68rem] font-semibold uppercase tracking-wide text-fg-muted">
        {label}
      </p>
      <p className="font-mono text-[1.1rem] font-bold text-fg mt-0.5">{value}</p>
      {detail && (
        <p className="text-[0.68rem] text-fg-muted mt-0.5">{detail}</p>
      )}
    </div>
  );
}
