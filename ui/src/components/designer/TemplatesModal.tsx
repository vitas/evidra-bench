import { useState, useMemo, useCallback, useEffect } from "react";
import type { Node, Edge } from "@xyflow/react";
import type { PuzzleMetadata } from "./yaml-generator";
import { SCENARIOS as ALL_SCENARIOS, type ScenarioMeta } from "../../data/catalog";
import { MODELS } from "../../data/models";
import { CATEGORY_COLORS, DIFFICULTY_COLORS } from "../../data/colors";

interface Template {
  nodes: Node[];
  edges: Edge[];
  metadata: PuzzleMetadata;
}

type ScenarioEntry = ScenarioMeta;

type CategoryFilter = "all" | "kubernetes" | "helm" | "argocd" | "terraform";
type DifficultyFilter = "all" | "easy" | "medium" | "hard";
type ModalMode = "run" | "new";

const EDGE_STYLE = { stroke: "var(--color-accent)", strokeWidth: 2, opacity: 0.7 };


const CATEGORY_LABELS: Record<CategoryFilter, string> = {
  all: "All",
  kubernetes: "Kubernetes",
  helm: "Helm",
  argocd: "ArgoCD",
  terraform: "Terraform",
};

const CHECK_TYPE_FOR_CATEGORY: Record<string, string> = {
  kubernetes: "deployment-ready",
  helm: "helm-release",
  argocd: "argocd-app-healthy",
  terraform: "deployment-ready",
};


function scenarioToTemplate(s: ScenarioEntry): Template {
  const resourceName = s.target.split("/")[1] || "web";
  const checkType = CHECK_TYPE_FOR_CATEGORY[s.category] || "deployment-ready";
  const timeLimit = s.difficulty === "easy" ? "5m" : s.difficulty === "medium" ? "8m" : "10m";

  return {
    nodes: [
      { id: "stack-1", type: "stack", position: { x: 50, y: 120 }, data: { kind: "stack", stackType: "web-app", namespace: "bench" } },
      { id: "break-1", type: "break", position: { x: 320, y: 120 }, data: { kind: "break", method: "kubectl-apply", action: s.breakType, target: s.target, customManifest: "" } },
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

function scenarioPath(s: ScenarioEntry): string {
  return `${s.category}/${s.id}`;
}

function formatCost(amount: number): string {
  if (amount < 0.01) return `<$0.01`;
  return `$${amount.toFixed(2)}`;
}

function generateBenchCommand(
  scenarios: string[],
  model: string,
  mode: "proxy" | "smart",
): string {
  const lines = ["infra-bench bench"];
  for (const s of scenarios) {
    lines.push(`  --scenario ${s}`);
  }
  lines.push(`  --model ${model}`);
  lines.push("  --provider bifrost");
  lines.push(mode === "smart" ? "  --smart-prescribe" : "  --proxy-mode");
  lines.push("  --reuse-cluster");
  lines.push("  --evidra-url $EVIDRA_URL");
  lines.push("  --evidra-api-key $EVIDRA_API_KEY");
  return lines.join(" \\\n");
}

const CATEGORY_COUNTS = ALL_SCENARIOS.reduce<Record<string, number>>((acc, s) => {
  acc[s.category] = (acc[s.category] || 0) + 1;
  return acc;
}, {});

interface TemplatesModalProps {
  open: boolean;
  onClose: () => void;
  onSelect: (template: Template) => void;
  initialMode?: ModalMode;
}

export function TemplatesModal({ open, onClose, onSelect, initialMode = "run" }: TemplatesModalProps) {
  const [modalMode, setModalMode] = useState<ModalMode>(initialMode);
  useEffect(() => { setModalMode(initialMode); }, [initialMode]);
  const [category, setCategory] = useState<CategoryFilter>("all");
  const [difficulty, setDifficulty] = useState<DifficultyFilter>("all");
  const [search, setSearch] = useState("");

  // Run mode state
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [selectedModel, setSelectedModel] = useState(MODELS[0].id);
  const [evidenceMode, setEvidenceMode] = useState<"proxy" | "smart">("proxy");
  const [generatedCommand, setGeneratedCommand] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const filtered = useMemo(() => {
    const q = search.toLowerCase();
    return ALL_SCENARIOS.filter((s) => {
      if (category !== "all" && s.category !== category) return false;
      if (difficulty !== "all" && s.difficulty !== difficulty) return false;
      if (q && !s.title.toLowerCase().includes(q) && !s.id.toLowerCase().includes(q)) return false;
      return true;
    });
  }, [category, difficulty, search]);

  const toggleScenario = useCallback((id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
    setGeneratedCommand(null);
    setCopied(false);
  }, []);

  const selectAll = useCallback(() => {
    setSelectedIds(new Set(filtered.map((s) => s.id)));
    setGeneratedCommand(null);
    setCopied(false);
  }, [filtered]);

  const clearSelection = useCallback(() => {
    setSelectedIds(new Set());
    setGeneratedCommand(null);
    setCopied(false);
  }, []);

  const handleGenerate = useCallback(() => {
    const scenarios = ALL_SCENARIOS
      .filter((s) => selectedIds.has(s.id))
      .map(scenarioPath);
    const cmd = generateBenchCommand(scenarios, selectedModel, evidenceMode);
    setGeneratedCommand(cmd);
    setCopied(false);
  }, [selectedIds, selectedModel, evidenceMode]);

  const handleCopy = useCallback(() => {
    if (generatedCommand) {
      navigator.clipboard.writeText(generatedCommand);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  }, [generatedCommand]);

  const selectedModelInfo = MODELS.find((m) => m.id === selectedModel) || MODELS[0];
  const estimatedCost = selectedIds.size * selectedModelInfo.costPerRun;

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
        {/* Header with mode toggle */}
        <div className="flex items-center justify-between px-5 py-3.5 border-b border-border-subtle shrink-0">
          <div className="flex items-center gap-3">
            <div className="flex bg-bg rounded-lg p-0.5 border border-border-subtle">
              <button
                onClick={() => setModalMode("run")}
                className={`text-[0.75rem] font-semibold px-3 py-1 rounded-md transition-all ${
                  modalMode === "run"
                    ? "bg-accent/15 text-accent shadow-sm"
                    : "text-fg-muted hover:text-fg"
                }`}
              >
                Run Benchmark
              </button>
              <button
                onClick={() => setModalMode("new")}
                className={`text-[0.75rem] font-medium px-3 py-1 rounded-md transition-all ${
                  modalMode === "new"
                    ? "bg-accent/15 text-accent shadow-sm"
                    : "text-fg-muted hover:text-fg"
                }`}
              >
                New Puzzle
              </button>
            </div>
          </div>
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

          {/* Difficulty pills + Select All/Clear for run mode */}
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

            {modalMode === "run" && (
              <div className="flex items-center gap-2">
                <button
                  onClick={selectAll}
                  className="text-[0.65rem] font-medium text-accent hover:text-accent/80 transition-colors"
                >
                  Select All
                </button>
                <span className="text-fg-muted/30 text-[0.6rem]">|</span>
                <button
                  onClick={clearSelection}
                  className="text-[0.65rem] font-medium text-fg-muted hover:text-fg transition-colors"
                >
                  Clear
                </button>
              </div>
            )}
          </div>
        </div>

        {/* Scenario list */}
        <div className="overflow-y-auto flex-1 min-h-0">
          {modalMode === "new" && (
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
          )}

          {filtered.length === 0 ? (
            <div className="px-5 py-8 text-center text-[0.8rem] text-fg-muted">
              No scenarios match the current filters.
            </div>
          ) : (
            <div className="divide-y divide-border-subtle">
              {filtered.map((s) => {
                const isSelected = selectedIds.has(s.id);

                if (modalMode === "new") {
                  return (
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
                  );
                }

                // Run mode: checkbox rows
                return (
                  <button
                    key={s.id}
                    onClick={() => toggleScenario(s.id)}
                    className={`w-full text-left px-5 py-2.5 transition-colors group flex items-center gap-3 ${
                      isSelected ? "bg-accent/5" : "hover:bg-bg-alt"
                    }`}
                  >
                    {/* Checkbox */}
                    <span
                      className={`w-4 h-4 rounded border flex items-center justify-center shrink-0 transition-all ${
                        isSelected
                          ? "bg-accent border-accent text-white"
                          : "border-border-subtle group-hover:border-accent/50"
                      }`}
                    >
                      {isSelected && (
                        <svg className="w-2.5 h-2.5" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth={2.5}>
                          <polyline points="2 6 5 9 10 3" />
                        </svg>
                      )}
                    </span>

                    {/* Difficulty badge */}
                    <span
                      className={`text-[0.6rem] font-semibold uppercase px-1.5 py-0.5 rounded shrink-0 w-[3.2rem] text-center ${DIFFICULTY_COLORS[s.difficulty]}`}
                    >
                      {s.difficulty}
                    </span>

                    {/* Title */}
                    <span className="text-[0.8rem] text-fg group-hover:text-accent transition-colors flex-1 min-w-0 truncate">
                      {s.title}
                    </span>

                    {/* Tags */}
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
                );
              })}
            </div>
          )}
        </div>

        {/* Footer: different per mode */}
        {modalMode === "run" ? (
          <div className="border-t border-border-subtle shrink-0">
            {/* Selected count + presets */}
            <div className="px-5 pt-3 pb-2 flex items-center gap-3 flex-wrap">
              <span className="text-[0.72rem] font-medium text-fg">
                {selectedIds.size} of {ALL_SCENARIOS.length} selected
              </span>
              {selectedIds.size > 0 && (
                <button
                  onClick={() => {
                    const name = window.prompt("Preset name:", `${selectedIds.size}-scenarios`);
                    if (!name) return;
                    const presets = JSON.parse(localStorage.getItem("bench-presets") || "[]");
                    presets.push({ name, ids: [...selectedIds] });
                    localStorage.setItem("bench-presets", JSON.stringify(presets));
                  }}
                  className="text-[0.68rem] text-accent hover:text-fg transition-colors"
                >
                  Save preset
                </button>
              )}
              {(JSON.parse(localStorage.getItem("bench-presets") || "[]") as { name: string; ids: string[] }[]).map((p, i) => (
                <span key={i} className="inline-flex items-center gap-0.5 text-[0.65rem] pl-2 pr-1 py-0.5 rounded-full border border-border text-fg-muted hover:border-accent transition-colors">
                  <button
                    onClick={() => setSelectedIds(new Set(p.ids))}
                    className="hover:text-fg transition-colors"
                    title={`Load ${p.ids.length} scenarios`}
                  >
                    {p.name}
                  </button>
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      const presets = JSON.parse(localStorage.getItem("bench-presets") || "[]") as { name: string; ids: string[] }[];
                      presets.splice(i, 1);
                      localStorage.setItem("bench-presets", JSON.stringify(presets));
                      setSelectedIds(new Set(selectedIds)); // force re-render
                    }}
                    className="text-fg-muted/50 hover:text-red-400 transition-colors ml-0.5"
                    title="Delete preset"
                  >
                    ✕
                  </button>
                </span>
              ))}
            </div>

            {selectedIds.size > 0 && (
              <div className="px-5 pb-4 space-y-3">
                {/* Model + Evidence mode row */}
                <div className="flex gap-3">
                  {/* Model dropdown */}
                  <div className="flex-1">
                    <label className="text-[0.68rem] font-semibold uppercase tracking-wider text-fg-muted mb-1 block">
                      Model
                    </label>
                    <select
                      value={selectedModel}
                      onChange={(e) => { setSelectedModel(e.target.value); setGeneratedCommand(null); }}
                      className="w-full bg-bg border border-border rounded-md px-3 py-1.5 text-[0.78rem] text-fg focus:outline-none focus:border-accent transition-colors appearance-none cursor-pointer"
                    >
                      {MODELS.map((m) => (
                        <option key={m.id} value={m.id}>
                          {m.label} (${m.costPerRun}/run)
                        </option>
                      ))}
                    </select>
                  </div>

                  {/* Evidence mode toggle */}
                  <div className="w-[180px]">
                    <label className="text-[0.68rem] font-semibold uppercase tracking-wider text-fg-muted mb-1 block">
                      Evidence
                    </label>
                    <div className="flex rounded-md border border-border overflow-hidden">
                      <button
                        onClick={() => { setEvidenceMode("proxy"); setGeneratedCommand(null); }}
                        className={`flex-1 px-2 py-1.5 text-[0.72rem] font-medium transition-all ${
                          evidenceMode === "proxy"
                            ? "bg-accent/15 text-accent"
                            : "text-fg-muted hover:text-fg bg-bg"
                        }`}
                      >
                        Proxy
                      </button>
                      <button
                        onClick={() => { setEvidenceMode("smart"); setGeneratedCommand(null); }}
                        className={`flex-1 px-2 py-1.5 text-[0.72rem] font-medium transition-all border-l border-border ${
                          evidenceMode === "smart"
                            ? "bg-accent/15 text-accent"
                            : "text-fg-muted hover:text-fg bg-bg"
                        }`}
                      >
                        Smart
                      </button>
                    </div>
                  </div>
                </div>

                {/* Cost estimate */}
                <div className="text-[0.68rem] text-fg-muted">
                  Estimated cost: <span className="text-fg font-medium">{formatCost(estimatedCost)}</span>
                  {" "}({selectedIds.size} scenarios x ${selectedModelInfo.costPerRun}/run)
                </div>

                {/* Generated command */}
                {generatedCommand ? (
                  <div>
                    <pre className="bg-bg text-[0.7rem] text-fg-muted p-3 rounded-md border border-border overflow-x-auto font-mono leading-relaxed max-h-[160px] overflow-y-auto">
                      {generatedCommand}
                    </pre>
                    <div className="flex items-center justify-between mt-2">
                      <span className="text-[0.65rem] text-fg-muted">
                        Copy and run locally. Remote execution coming soon.
                      </span>
                      <button
                        onClick={handleCopy}
                        className="px-3 py-1 bg-accent text-white text-[0.72rem] font-semibold rounded-md hover:bg-accent/80 transition-all"
                      >
                        {copied ? "Copied!" : "Copy"}
                      </button>
                    </div>
                  </div>
                ) : (
                  <button
                    onClick={handleGenerate}
                    className="w-full inline-flex items-center justify-center gap-1.5 px-4 py-2 bg-accent text-white text-[0.78rem] font-semibold rounded-md hover:bg-accent/80 transition-all"
                  >
                    <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.5}>
                      <polyline points="4 17 10 11 4 5" />
                      <line x1="12" y1="19" x2="20" y2="19" />
                    </svg>
                    Generate Run Command
                  </button>
                )}
              </div>
            )}
          </div>
        ) : (
          <div className="px-5 py-2 border-t border-border-subtle shrink-0">
            <span className="text-[0.68rem] text-fg-muted">
              {filtered.length} of {ALL_SCENARIOS.length} scenarios
            </span>
          </div>
        )}
      </div>
    </div>
  );
}
