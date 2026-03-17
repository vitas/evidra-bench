import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router";
import { useApi } from "../hooks/useApi";

interface Scenario {
  id: string;
  title: string;
  category: string;
  tags: string[];
  chaos: boolean;
  evidra: boolean;
}

interface ScenariosResponse {
  items: Scenario[];
  total: number;
}

interface ScenarioStat {
  scenario_id: string;
  runs: number;
  passed: number;
}

interface Stats {
  total_runs: number;
  pass_count: number;
  fail_count: number;
  by_scenario: ScenarioStat[];
}

const CATEGORIES = ["All", "kubectl", "helm", "argocd", "terraform"] as const;
const FEATURES = ["All", "Chaos enabled", "Evidra enabled"] as const;

export function Scenarios() {
  const { request } = useApi();
  const [data, setData] = useState<ScenariosResponse | null>(null);
  const [stats, setStats] = useState<Map<string, ScenarioStat>>(new Map());
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [search, setSearch] = useState("");
  const [category, setCategory] = useState<string>("All");
  const [feature, setFeature] = useState<string>("All");

  useEffect(() => {
    Promise.all([
      request<ScenariosResponse>("/v1/bench/scenarios"),
      request<Stats>("/v1/bench/stats"),
    ])
      .then(([scenarios, st]) => {
        setData(scenarios);
        const map = new Map<string, ScenarioStat>();
        for (const s of st.by_scenario ?? []) {
          map.set(s.scenario_id, s);
        }
        setStats(map);
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [request]);

  const filtered = useMemo(() => {
    if (!data) return [];
    return data.items.filter((s) => {
      if (search) {
        const q = search.toLowerCase();
        if (!s.id.toLowerCase().includes(q) && !s.title.toLowerCase().includes(q)) return false;
      }
      if (category !== "All" && s.category !== category) return false;
      if (feature === "Chaos enabled" && !s.chaos) return false;
      if (feature === "Evidra enabled" && !s.evidra) return false;
      return true;
    });
  }, [data, search, category, feature]);

  const grouped = useMemo(() => {
    const groups = new Map<string, Scenario[]>();
    for (const s of filtered) {
      const list = groups.get(s.category) ?? [];
      list.push(s);
      groups.set(s.category, list);
    }
    return groups;
  }, [filtered]);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20 text-fg-muted text-[0.85rem]">
        Loading scenarios...
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center py-20 text-danger text-[0.85rem]">
        Failed to load scenarios: {error}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      {/* Header */}
      <div>
        <h1 className="text-xl font-bold text-fg">Scenarios</h1>
        <p className="text-[0.83rem] text-fg-muted mt-1">
          {data?.total ?? 0} scenarios across kubectl, Helm, Argo CD, and Terraform
        </p>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap items-center gap-3">
        <input
          type="text"
          placeholder="Search by ID or title..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="h-9 px-3 text-[0.83rem] text-fg bg-bg-elevated border border-border-subtle rounded-lg placeholder:text-fg-muted/50 focus:outline-none focus:border-accent transition-colors w-64"
        />

        <select
          value={category}
          onChange={(e) => setCategory(e.target.value)}
          className="h-9 px-3 text-[0.83rem] text-fg bg-bg-elevated border border-border-subtle rounded-lg focus:outline-none focus:border-accent transition-colors cursor-pointer"
        >
          {CATEGORIES.map((c) => (
            <option key={c} value={c}>
              {c === "All" ? "All categories" : c}
            </option>
          ))}
        </select>

        <select
          value={feature}
          onChange={(e) => setFeature(e.target.value)}
          className="h-9 px-3 text-[0.83rem] text-fg bg-bg-elevated border border-border-subtle rounded-lg focus:outline-none focus:border-accent transition-colors cursor-pointer"
        >
          {FEATURES.map((f) => (
            <option key={f} value={f}>
              {f === "All" ? "All features" : f}
            </option>
          ))}
        </select>
      </div>

      {/* Empty state */}
      {filtered.length === 0 && (
        <div className="flex items-center justify-center py-16 text-fg-muted text-[0.85rem]">
          No scenarios match the current filters.
        </div>
      )}

      {/* Grouped cards */}
      {Array.from(grouped.entries()).map(([cat, scenarios]) => (
        <section key={cat} className="flex flex-col gap-3">
          <h2 className="text-[0.85rem] font-semibold text-fg-muted uppercase tracking-wide">
            {cat}
            <span className="ml-2 font-normal normal-case tracking-normal">
              ({scenarios.length})
            </span>
          </h2>

          <div className="grid grid-cols-[repeat(auto-fill,minmax(320px,1fr))] gap-4">
            {scenarios.map((s) => (
              <ScenarioCard key={s.id} scenario={s} stat={stats.get(s.id)} />
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}

function ScenarioCard({ scenario, stat }: { scenario: Scenario; stat?: ScenarioStat }) {
  const passRate = stat && stat.runs > 0
    ? Math.round((stat.passed / stat.runs) * 100)
    : null;

  return (
    <Link
      to={`/runs?scenario=${scenario.id}`}
      className="bg-bg-elevated border border-border-subtle rounded-[10px] p-4 hover:border-accent hover:shadow-[var(--shadow-card-lg)] hover:-translate-y-px transition-all cursor-pointer flex flex-col gap-2"
      style={{ textDecoration: "none", color: "inherit" }}
    >
      <div className="flex flex-col gap-1">
        <span className="text-[0.85rem] font-bold text-fg">{scenario.title}</span>
        <span className="font-mono text-[0.73rem] text-fg-muted">{scenario.id}</span>
      </div>

      {scenario.tags.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {scenario.tags.map((tag) => (
            <span
              key={tag}
              className="bg-accent-subtle text-fg-muted font-medium text-[0.72rem] px-2 py-0.5 rounded"
            >
              {tag}
            </span>
          ))}
          {scenario.chaos && (
            <span className="bg-warning-tint text-warning font-medium text-[0.72rem] px-2 py-0.5 rounded">
              chaos
            </span>
          )}
          {scenario.evidra && (
            <span className="bg-info-tint text-info font-medium text-[0.72rem] px-2 py-0.5 rounded">
              evidra
            </span>
          )}
        </div>
      )}

      <div className="mt-auto pt-3 border-t border-border-subtle flex items-center gap-4 font-mono text-[0.73rem] text-fg-muted">
        {stat ? (
          <>
            <span>
              <strong className="text-fg">{stat.runs}</strong> runs
            </span>
            <span>
              <strong className="text-fg">{stat.passed}</strong>/{stat.runs} passed
            </span>
            <span
              className={`font-semibold ${
                passRate !== null && passRate >= 70
                  ? "text-accent"
                  : passRate !== null && passRate >= 40
                    ? "text-warning"
                    : "text-danger"
              }`}
            >
              {passRate}%
            </span>
          </>
        ) : (
          <span className="text-fg-muted">No runs yet</span>
        )}
      </div>
    </Link>
  );
}
