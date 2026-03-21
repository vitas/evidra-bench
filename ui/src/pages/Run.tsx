import { useState, useMemo, useCallback } from "react";
import { SCENARIOS, type ScenarioMeta } from "../data/catalog";
import { useEvidenceMode } from "../hooks/useEvidenceMode";

const MODELS = [
  { id: "gemini-2.5-flash", label: "Gemini 2.5 Flash", cost: "$0.001/run" },
  { id: "gpt-4.1", label: "GPT-4.1", cost: "$0.08/run" },
  { id: "gpt-4o", label: "GPT-4o", cost: "$0.03/run" },
  { id: "claude-sonnet-4-20250514", label: "Claude Sonnet 4", cost: "$0.24/run" },
  { id: "gpt-5.2", label: "GPT-5.2", cost: "$0.10/run" },
  { id: "qwen-plus", label: "Qwen Plus", cost: "$0.02/run" },
];

type Category = "all" | ScenarioMeta["category"];

const CATEGORY_PILLS: { value: Category; label: string }[] = [
  { value: "all", label: "All" },
  { value: "kubernetes", label: "kubectl" },
  { value: "helm", label: "Helm" },
  { value: "argocd", label: "Argo CD" },
  { value: "terraform", label: "Terraform" },
];

const DIFFICULTY_COLORS: Record<ScenarioMeta["difficulty"], string> = {
  easy: "bg-green-500/15 text-green-400 border-green-500/30",
  medium: "bg-yellow-500/15 text-yellow-400 border-yellow-500/30",
  hard: "bg-red-500/15 text-red-400 border-red-500/30",
};

const CATEGORY_COLORS: Record<ScenarioMeta["category"], string> = {
  kubernetes: "bg-blue-500/15 text-blue-400 border-blue-500/30",
  helm: "bg-purple-500/15 text-purple-400 border-purple-500/30",
  argocd: "bg-orange-500/15 text-orange-400 border-orange-500/30",
  terraform: "bg-teal-500/15 text-teal-400 border-teal-500/30",
};

const CATEGORY_LABELS: Record<ScenarioMeta["category"], string> = {
  kubernetes: "kubectl",
  helm: "Helm",
  argocd: "Argo CD",
  terraform: "Terraform",
};

export function Run() {
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState<Category>("all");
  const [selectedModel, setSelectedModel] = useState(MODELS[0].id);
  const { mode, setMode } = useEvidenceMode();
  const [copied, setCopied] = useState(false);

  const filtered = useMemo(() => {
    let list = SCENARIOS;
    if (category !== "all") {
      list = list.filter((s) => s.category === category);
    }
    if (search.trim()) {
      const q = search.toLowerCase();
      list = list.filter(
        (s) =>
          s.id.toLowerCase().includes(q) ||
          s.title.toLowerCase().includes(q) ||
          s.description.toLowerCase().includes(q),
      );
    }
    return list;
  }, [category, search]);

  const toggleScenario = useCallback((id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const selectAll = useCallback(() => {
    setSelectedIds(new Set(filtered.map((s) => s.id)));
  }, [filtered]);

  const clearAll = useCallback(() => {
    setSelectedIds(new Set());
  }, []);

  const command = useMemo(() => {
    if (selectedIds.size === 0) return null;
    const scenarioList = [...selectedIds].join(",");
    const lines = [
      "infra-bench bench \\",
      `  --scenarios ${scenarioList} \\`,
      `  --model ${selectedModel} \\`,
      "  --provider bifrost \\",
      mode === "smart" ? "  --smart-prescribe \\" : "  --proxy-mode \\",
      "  --reuse-cluster \\",
      "  --timeout 5m \\",
      "  --evidra-url $EVIDRA_URL \\",
      "  --evidra-api-key $EVIDRA_API_KEY",
    ];
    return lines.join("\n");
  }, [selectedIds, selectedModel, mode]);

  const handleCopy = useCallback(() => {
    if (!command) return;
    navigator.clipboard.writeText(command).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }, [command]);

  return (
    <div className="max-w-7xl mx-auto space-y-6">
      {/* Page header */}
      <div>
        <h1 className="text-xl font-bold text-fg">Run Benchmark</h1>
        <p className="text-[0.82rem] text-fg-muted mt-1">
          Select scenarios, pick a model, and generate a CLI command to run
          locally.
        </p>
      </div>

      {/* Two-column layout */}
      <div className="grid grid-cols-1 lg:grid-cols-[1fr_380px] gap-6">
        {/* Section 1: Scenario Selection */}
        <div className="bg-bg-elevated border border-border rounded-xl overflow-hidden flex flex-col">
          {/* Toolbar */}
          <div className="px-4 py-3 border-b border-border-subtle space-y-3">
            {/* Search */}
            <div className="relative">
              <svg
                className="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-fg-muted"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth={2}
              >
                <circle cx="11" cy="11" r="8" />
                <line x1="21" y1="21" x2="16.65" y2="16.65" />
              </svg>
              <input
                type="text"
                placeholder="Search scenarios..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="w-full pl-9 pr-3 py-1.5 text-[0.82rem] bg-bg border border-border-subtle rounded-md text-fg placeholder:text-fg-muted/50 focus:outline-none focus:border-accent/60"
              />
            </div>

            {/* Category pills + actions */}
            <div className="flex items-center justify-between gap-3">
              <div className="flex gap-1.5 flex-wrap">
                {CATEGORY_PILLS.map((pill) => (
                  <button
                    key={pill.value}
                    onClick={() => setCategory(pill.value)}
                    className={`px-2.5 py-0.5 rounded-full text-[0.72rem] font-medium border transition-all ${
                      category === pill.value
                        ? "border-accent bg-accent/10 text-accent"
                        : "border-border-subtle text-fg-muted hover:border-accent/40 hover:text-fg"
                    }`}
                  >
                    {pill.label}
                  </button>
                ))}
              </div>
              <div className="flex items-center gap-2 shrink-0">
                <span className="text-[0.72rem] text-fg-muted">
                  {selectedIds.size} of {SCENARIOS.length} selected
                </span>
                <button
                  onClick={selectAll}
                  className="text-[0.72rem] font-medium text-accent hover:text-accent/80 transition-colors"
                >
                  Select All
                </button>
                <button
                  onClick={clearAll}
                  className="text-[0.72rem] font-medium text-fg-muted hover:text-fg transition-colors"
                >
                  Clear
                </button>
              </div>
            </div>
          </div>

          {/* Scenario list */}
          <div className="overflow-y-auto max-h-[480px] divide-y divide-border-subtle">
            {filtered.length === 0 ? (
              <div className="px-4 py-8 text-center text-[0.82rem] text-fg-muted">
                No scenarios match your search.
              </div>
            ) : (
              filtered.map((s) => (
                <label
                  key={s.id}
                  className={`flex items-center gap-3 px-4 py-2.5 cursor-pointer transition-colors hover:bg-accent/5 ${
                    selectedIds.has(s.id) ? "bg-accent/5" : ""
                  }`}
                >
                  <input
                    type="checkbox"
                    checked={selectedIds.has(s.id)}
                    onChange={() => toggleScenario(s.id)}
                    className="accent-[var(--color-accent)] w-3.5 h-3.5 shrink-0"
                  />
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-[0.82rem] font-medium text-fg truncate">
                        {s.title}
                      </span>
                    </div>
                    <p className="text-[0.72rem] text-fg-muted truncate mt-0.5">
                      {s.description}
                    </p>
                  </div>
                  <div className="flex items-center gap-1.5 shrink-0">
                    <span
                      className={`px-1.5 py-0.5 rounded text-[0.65rem] font-medium border ${CATEGORY_COLORS[s.category]}`}
                    >
                      {CATEGORY_LABELS[s.category]}
                    </span>
                    <span
                      className={`px-1.5 py-0.5 rounded text-[0.65rem] font-medium border ${DIFFICULTY_COLORS[s.difficulty]}`}
                    >
                      {s.difficulty}
                    </span>
                    {s.chaos && (
                      <span className="px-1.5 py-0.5 rounded text-[0.65rem] font-medium border bg-pink-500/15 text-pink-400 border-pink-500/30">
                        chaos
                      </span>
                    )}
                  </div>
                </label>
              ))
            )}
          </div>
        </div>

        {/* Section 2: Configuration */}
        <div className="space-y-4">
          {/* Model picker */}
          <div className="bg-bg-elevated border border-border rounded-xl p-4">
            <label className="text-[0.72rem] font-semibold uppercase tracking-wider text-fg-muted mb-2.5 block">
              Model
            </label>
            <div className="grid grid-cols-2 gap-2">
              {MODELS.map((m) => (
                <button
                  key={m.id}
                  onClick={() => setSelectedModel(m.id)}
                  className={`text-left px-3 py-2 rounded-md border text-[0.78rem] transition-all ${
                    selectedModel === m.id
                      ? "border-accent bg-accent/10 text-fg"
                      : "border-border text-fg-muted hover:border-accent/50"
                  }`}
                >
                  <div className="font-medium">{m.label}</div>
                  <div className="text-[0.68rem] text-fg-muted">{m.cost}</div>
                </button>
              ))}
            </div>
          </div>

          {/* Evidence mode */}
          <div className="bg-bg-elevated border border-border rounded-xl p-4">
            <label className="text-[0.72rem] font-semibold uppercase tracking-wider text-fg-muted mb-2.5 block">
              Evidence Mode
            </label>
            <div className="flex gap-2">
              <button
                onClick={() => setMode("proxy")}
                className={`flex-1 px-3 py-2 rounded-md border text-[0.78rem] font-medium transition-all ${
                  mode === "proxy"
                    ? "border-accent bg-accent/10 text-fg"
                    : "border-border text-fg-muted hover:border-accent/50"
                }`}
              >
                Proxy
                <span className="block text-[0.65rem] font-normal text-fg-muted">
                  Zero overhead
                </span>
              </button>
              <button
                onClick={() => setMode("smart")}
                className={`flex-1 px-3 py-2 rounded-md border text-[0.78rem] font-medium transition-all ${
                  mode === "smart"
                    ? "border-accent bg-accent/10 text-fg"
                    : "border-border text-fg-muted hover:border-accent/50"
                }`}
              >
                Smart Prescribe
                <span className="block text-[0.65rem] font-normal text-fg-muted">
                  Risk assessment
                </span>
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Section 3: Generated Command */}
      <div className="bg-bg-elevated border border-border rounded-xl overflow-hidden">
        <div className="flex items-center justify-between px-4 py-3 border-b border-border-subtle">
          <label className="text-[0.72rem] font-semibold uppercase tracking-wider text-fg-muted">
            Generated Command
          </label>
          {command && (
            <button
              onClick={handleCopy}
              className="inline-flex items-center gap-1.5 px-3 py-1 bg-accent text-white text-[0.72rem] font-semibold rounded-md hover:bg-accent/80 transition-all"
            >
              {copied ? (
                <>
                  <svg
                    className="w-3 h-3"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth={2.5}
                  >
                    <polyline points="20 6 9 17 4 12" />
                  </svg>
                  Copied
                </>
              ) : (
                <>
                  <svg
                    className="w-3 h-3"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth={2}
                  >
                    <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                    <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1" />
                  </svg>
                  Copy
                </>
              )}
            </button>
          )}
        </div>
        <div className="px-4 py-3">
          {command ? (
            <pre className="text-[0.75rem] text-fg font-mono leading-relaxed whitespace-pre overflow-x-auto">
              {command}
            </pre>
          ) : (
            <p className="text-[0.82rem] text-fg-muted py-4 text-center">
              Select at least one scenario to generate a command.
            </p>
          )}
        </div>
        {command && (
          <div className="px-4 pb-3">
            <p className="text-[0.72rem] text-fg-muted">
              Run this command locally.{" "}
              <a
                href="https://evidra.cc/bench"
                target="_blank"
                rel="noopener noreferrer"
                className="text-accent hover:text-accent/80 transition-colors"
              >
                View results at evidra.cc/bench
              </a>{" "}
              after the run completes.
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
