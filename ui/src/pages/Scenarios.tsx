import { useState, useMemo } from "react";
import { useNavigate } from "react-router";
import { SCENARIOS, CATEGORY_LABELS, type ScenarioMeta } from "../data/catalog";
import { CATEGORY_COLORS, DIFFICULTY_COLORS } from "../data/colors";

type CategoryFilter = "all" | "kubernetes" | "helm" | "argocd" | "terraform";
type DifficultyFilter = "all" | "easy" | "medium" | "hard";

const CATEGORY_COUNTS = SCENARIOS.reduce<Record<string, number>>((acc, s) => {
  acc[s.category] = (acc[s.category] || 0) + 1;
  return acc;
}, {});

const CATEGORY_PILLS: { key: CategoryFilter; label: string }[] = [
  { key: "all", label: `All (${SCENARIOS.length})` },
  { key: "kubernetes", label: `${CATEGORY_LABELS["kubernetes"]} (${CATEGORY_COUNTS["kubernetes"] || 0})` },
  { key: "helm", label: `${CATEGORY_LABELS["helm"]} (${CATEGORY_COUNTS["helm"] || 0})` },
  { key: "argocd", label: `${CATEGORY_LABELS["argocd"]} (${CATEGORY_COUNTS["argocd"] || 0})` },
  { key: "terraform", label: `${CATEGORY_LABELS["terraform"]} (${CATEGORY_COUNTS["terraform"] || 0})` },
];

const DIFFICULTY_PILLS: DifficultyFilter[] = ["all", "easy", "medium", "hard"];


function ScenarioCard({ scenario }: { scenario: ScenarioMeta }) {
  const navigate = useNavigate();

  return (
    <button
      onClick={() => navigate(`/designer?scenario=${scenario.id}`)}
      className="bg-bg-elevated border border-border rounded-xl p-5 text-left hover:border-accent/50 transition-all group flex flex-col gap-3"
    >
      {/* Badges row */}
      <div className="flex items-center gap-2">
        <span
          className={`text-[0.65rem] font-semibold px-2 py-0.5 rounded-full border ${CATEGORY_COLORS[scenario.category]}`}
        >
          {CATEGORY_LABELS[scenario.category]}
        </span>
        <span
          className={`text-[0.65rem] font-semibold uppercase px-2 py-0.5 rounded-full ${DIFFICULTY_COLORS[scenario.difficulty]}`}
        >
          {scenario.difficulty}
        </span>
        {scenario.chaos && (
          <span className="text-[0.65rem] font-semibold px-2 py-0.5 rounded-full bg-red-500/20 text-red-400 border border-red-500/20">
            chaos
          </span>
        )}
      </div>

      {/* Title */}
      <h3 className="text-[0.88rem] font-semibold text-fg group-hover:text-accent transition-colors leading-snug">
        {scenario.title}
      </h3>

      {/* Description */}
      <p className="text-[0.78rem] text-fg-muted leading-relaxed line-clamp-2">
        {scenario.description}
      </p>

      {/* Target */}
      <div className="mt-auto pt-1">
        <span className="text-[0.68rem] font-mono text-fg-muted/60">
          {scenario.target}
        </span>
      </div>
    </button>
  );
}

export function Scenarios() {
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState<CategoryFilter>("all");
  const [difficulty, setDifficulty] = useState<DifficultyFilter>("all");

  const filtered = useMemo(() => {
    const q = search.toLowerCase();
    return SCENARIOS.filter((s) => {
      if (category !== "all" && s.category !== category) return false;
      if (difficulty !== "all" && s.difficulty !== difficulty) return false;
      if (
        q &&
        !s.title.toLowerCase().includes(q) &&
        !s.description.toLowerCase().includes(q)
      )
        return false;
      return true;
    });
  }, [search, category, difficulty]);

  return (
    <div className="max-w-6xl mx-auto px-6 py-10">
      {/* Header */}
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-fg">Scenario Catalog</h1>
        <p className="text-fg-muted mt-1.5 text-[0.88rem]">
          {SCENARIOS.length} benchmark scenarios for AI infrastructure agents
        </p>
      </div>

      {/* Search */}
      <div className="mb-5">
        <input
          type="text"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search by title or description..."
          className="w-full bg-bg-elevated border border-border rounded-lg px-4 py-2.5 text-[0.85rem] text-fg placeholder:text-fg-muted/50 focus:outline-none focus:border-accent transition-colors"
        />
      </div>

      {/* Filter row */}
      <div className="flex flex-col sm:flex-row sm:items-center gap-3 mb-6">
        {/* Category pills */}
        <div className="flex flex-wrap gap-1.5">
          {CATEGORY_PILLS.map((c) => {
            const active = category === c.key;
            return (
              <button
                key={c.key}
                onClick={() => setCategory(c.key)}
                className={`text-[0.75rem] font-medium px-3 py-1.5 rounded-lg transition-colors ${
                  active
                    ? "bg-accent/15 text-accent"
                    : "text-fg-muted hover:text-fg hover:bg-bg-elevated"
                }`}
              >
                {c.label}
              </button>
            );
          })}
        </div>

        {/* Separator */}
        <div className="hidden sm:block w-px h-5 bg-border" />

        {/* Difficulty pills */}
        <div className="flex gap-1.5">
          {DIFFICULTY_PILLS.map((d) => {
            const active = difficulty === d;
            const colors =
              d === "all"
                ? active
                  ? "bg-fg/10 text-fg"
                  : "text-fg-muted hover:text-fg"
                : active
                  ? DIFFICULTY_COLORS[d]
                  : "text-fg-muted hover:text-fg";
            return (
              <button
                key={d}
                onClick={() => setDifficulty(d)}
                className={`text-[0.72rem] font-medium px-2.5 py-1 rounded-full transition-colors capitalize ${colors}`}
              >
                {d === "all" ? "All" : d}
              </button>
            );
          })}
        </div>
      </div>

      {/* Results count */}
      <div className="mb-4">
        <span className="text-[0.75rem] text-fg-muted">
          {filtered.length} scenario{filtered.length !== 1 ? "s" : ""}
          {category !== "all" || difficulty !== "all" || search
            ? " matching filters"
            : ""}
        </span>
      </div>

      {/* Grid */}
      {filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="text-4xl mb-4 opacity-30">
            <svg
              className="w-12 h-12 text-fg-muted"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth={1.5}
            >
              <circle cx="11" cy="11" r="8" />
              <line x1="21" y1="21" x2="16.65" y2="16.65" />
            </svg>
          </div>
          <p className="text-fg-muted text-[0.9rem] font-medium">
            No scenarios match your filters
          </p>
          <p className="text-fg-muted/60 text-[0.8rem] mt-1">
            Try adjusting the search term or clearing filters
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {filtered.map((s) => (
            <ScenarioCard key={s.id} scenario={s} />
          ))}
        </div>
      )}
    </div>
  );
}
