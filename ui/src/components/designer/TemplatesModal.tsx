import { useState, useMemo } from "react";
import type { Node, Edge } from "@xyflow/react";
import type { PuzzleMetadata } from "./yaml-generator";
import { SCENARIOS as ALL_SCENARIOS, CATEGORY_LABELS, type ScenarioMeta } from "../../data/catalog";
import { CATEGORY_COLORS, DIFFICULTY_COLORS } from "../../data/colors";

interface Template {
  nodes: Node[];
  edges: Edge[];
  metadata: PuzzleMetadata;
}

type CategoryFilter = "all" | "kubernetes" | "helm" | "argocd" | "terraform";
type DifficultyFilter = "all" | "easy" | "medium" | "hard";

const EDGE_STYLE = { stroke: "var(--color-accent)", strokeWidth: 2, opacity: 0.7 };

const CHECK_TYPE_FOR_CATEGORY: Record<string, string> = {
  kubernetes: "deployment-ready",
  helm: "helm-release",
  argocd: "argocd-app-healthy",
  terraform: "command-succeeds",
  aws: "command-succeeds",
};

function scenarioToTemplate(s: ScenarioMeta): Template {
  const resourceName = s.target.split("/")[1] || "web";
  const checkType = CHECK_TYPE_FOR_CATEGORY[s.category] || "deployment-ready";
  const timeLimit = s.difficulty === "easy" ? "5m" : s.difficulty === "medium" ? "8m" : "10m";

  return {
    nodes: [
      { id: "stack-1", type: "stack", position: { x: 50, y: 120 }, data: { kind: "stack", stackType: "web-app", namespace: "bench" } },
      { id: "break-1", type: "break", position: { x: 320, y: 120 }, data: { kind: "break", method: (s.category === "terraform" || s.category === "aws") ? "script" : "kubectl-apply", action: s.breakType === "custom" || s.breakType === "multi-stage" || s.breakType === "shell" ? "custom" : s.breakType, target: s.target, customManifest: "" } },
      { id: "verify-1", type: "verify", position: { x: 590, y: 120 }, data: { kind: "verify", checkType, namespace: "bench", resourceName } },
    ],
    edges: [
      { id: "e-stack-break", source: "stack-1", target: "break-1", animated: true, style: EDGE_STYLE },
      { id: "e-break-verify", source: "break-1", target: "verify-1", animated: true, style: EDGE_STYLE },
    ],
    metadata: {
      name: s.id,
      title: s.title,
      description: "",
      difficulty: s.difficulty,
      timeLimit,
      category: s.category,
    },
  };
}

const CATEGORY_COUNTS = ALL_SCENARIOS.reduce<Record<string, number>>((acc, s) => {
  acc[s.category] = (acc[s.category] || 0) + 1;
  return acc;
}, {});

interface TemplatesModalProps {
  open: boolean;
  onClose: () => void;
  onSelect: (template: Template) => void;
}

export function TemplatesModal({ open, onClose, onSelect }: TemplatesModalProps) {
  const [category, setCategory] = useState<CategoryFilter>("all");
  const [difficulty, setDifficulty] = useState<DifficultyFilter>("all");
  const [search, setSearch] = useState("");

  const filtered = useMemo(() => {
    const q = search.toLowerCase();
    return ALL_SCENARIOS.filter((s) => {
      if (category !== "all" && s.category !== category) return false;
      if (difficulty !== "all" && s.difficulty !== difficulty) return false;
      if (q && !s.title.toLowerCase().includes(q) && !s.id.toLowerCase().includes(q)) return false;
      return true;
    });
  }, [category, difficulty, search]);

  if (!open) return null;

  const categories: CategoryFilter[] = ["all", "kubernetes", "helm", "argocd", "terraform"];
  const difficulties: DifficultyFilter[] = ["all", "easy", "medium", "hard"];

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm"
      onClick={onClose}
    >
      <div
        className="bg-bg-elevated border border-border rounded-xl shadow-2xl w-[780px] max-w-[90vw] max-h-[85vh] overflow-hidden flex flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-3.5 border-b border-border-subtle shrink-0">
          <span className="text-[0.8rem] font-semibold text-fg">Scenarios</span>
          <button
            onClick={onClose}
            className="text-fg-muted hover:text-fg text-lg transition-colors leading-none"
          >
            &#x2715;
          </button>
        </div>

        {/* Filters */}
        <div className="px-5 pt-4 pb-3 border-b border-border-subtle space-y-3 shrink-0">
          {/* Search */}
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search scenarios..."
            className="w-full bg-bg border border-border rounded-lg px-3 py-1.5 text-[0.8rem] text-fg placeholder:text-fg-muted/50 focus:outline-none focus:border-accent transition-colors"
          />

          {/* Category tabs */}
          <div className="flex gap-1">
            {categories.map((c) => {
              const count = c === "all" ? ALL_SCENARIOS.length : (CATEGORY_COUNTS[c] || 0);
              const active = category === c;
              return (
                <button
                  key={c}
                  onClick={() => setCategory(c)}
                  className={`text-[0.7rem] font-medium px-2.5 py-1 rounded-md transition-colors ${
                    active
                      ? "bg-accent/15 text-accent"
                      : "text-fg-muted hover:text-fg hover:bg-bg-alt"
                  }`}
                >
                  {CATEGORY_LABELS[c]} ({count})
                </button>
              );
            })}
          </div>

          {/* Difficulty pills */}
          <div className="flex items-center justify-between">
            <div className="flex gap-1.5">
              {difficulties.map((d) => {
                const active = difficulty === d;
                const colors = d === "all"
                  ? (active ? "bg-fg/10 text-fg" : "text-fg-muted hover:text-fg")
                  : (active ? DIFFICULTY_COLORS[d] : "text-fg-muted hover:text-fg");
                return (
                  <button
                    key={d}
                    onClick={() => setDifficulty(d)}
                    className={`text-[0.65rem] font-medium px-2 py-0.5 rounded-full transition-colors ${colors}`}
                  >
                    {d === "all" ? "All" : d}
                  </button>
                );
              })}
            </div>
          </div>
        </div>

        {/* Scenario list */}
        <div className="overflow-y-auto flex-1 min-h-0">
          <button
            onClick={() => {
              onSelect({
                nodes: [],
                edges: [],
                metadata: {
                  name: "my-puzzle",
                  title: "New Puzzle",
                  description: "",
                  difficulty: "medium",
                  timeLimit: "8m",
                  category: "kubernetes",
                },
              });
            }}
            className="w-full text-left px-5 py-2.5 hover:bg-bg-alt transition-colors group flex items-center gap-3 border-b border-border-subtle"
          >
            <span className="text-[0.6rem] font-semibold uppercase px-1.5 py-0.5 rounded shrink-0 w-[3.2rem] text-center bg-fg/10 text-fg-muted">
              ---
            </span>
            <span className="text-[0.8rem] text-fg-muted group-hover:text-accent transition-colors flex-1 min-w-0 truncate italic">
              Blank Canvas
            </span>
          </button>

          {filtered.length === 0 ? (
            <div className="px-5 py-8 text-center text-[0.8rem] text-fg-muted">
              No scenarios match the current filters.
            </div>
          ) : (
            <div className="divide-y divide-border-subtle">
              {filtered.map((s) => (
                <button
                  key={s.id}
                  onClick={() => onSelect(scenarioToTemplate(s))}
                  className="w-full text-left px-5 py-2.5 hover:bg-bg-alt transition-colors group flex items-center gap-3"
                >
                  <span
                    className={`text-[0.6rem] font-semibold uppercase px-1.5 py-0.5 rounded shrink-0 w-[3.2rem] text-center ${DIFFICULTY_COLORS[s.difficulty]}`}
                  >
                    {s.difficulty}
                  </span>
                  <span className="text-[0.8rem] text-fg group-hover:text-accent transition-colors flex-1 min-w-0 truncate">
                    {s.title}
                  </span>
                  <span className="flex items-center gap-1.5 shrink-0">
                    <span className={`text-[0.6rem] font-medium px-1.5 py-0.5 rounded-full ${CATEGORY_COLORS[s.category]}`}>
                      {s.category}
                    </span>
                    {s.chaos && (
                      <span className="text-[0.6rem] font-medium px-1.5 py-0.5 rounded-full bg-red-500/20 text-red-400">
                        chaos
                      </span>
                    )}
                  </span>
                </button>
              ))}
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="px-5 py-2 border-t border-border-subtle shrink-0">
          <span className="text-[0.68rem] text-fg-muted">
            {filtered.length} of {ALL_SCENARIOS.length} scenarios
          </span>
        </div>
      </div>
    </div>
  );
}
