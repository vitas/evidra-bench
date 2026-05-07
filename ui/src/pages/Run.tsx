import { useState, useMemo, useCallback, useRef, useEffect } from "react";
import { SCENARIOS, CATEGORY_LABELS, TRACK_LABELS, type ScenarioMeta } from "../data/catalog";
import { CATEGORY_COLORS, DIFFICULTY_COLORS, LEVEL_COLORS } from "../data/colors";
import { useEvidenceMode } from "../hooks/useEvidenceMode";
import { buildBenchCommand, EVIDENCE_MODES } from "../lib/commandBuilder.mts";
import { useBenchApi } from "../hooks/useBenchApi";
import { useModels } from "../hooks/useModels";

type Category = "all" | ScenarioMeta["category"];
type Track = "all" | ScenarioMeta["track"];

type TriggerState =
  | { phase: "idle" }
  | { phase: "triggering" }
  | { phase: "running"; jobId: string; completed: number; total: number; status: string }
  | { phase: "done"; jobId: string; status: string; completed: number; total: number }
  | { phase: "error"; message: string };

const CATEGORY_PILLS: { value: Category; label: string }[] = [
  { value: "all", label: "All" },
  { value: "kubernetes", label: CATEGORY_LABELS["kubernetes"] },
  { value: "helm", label: CATEGORY_LABELS["helm"] },
  { value: "argocd", label: CATEGORY_LABELS["argocd"] },
  { value: "terraform", label: CATEGORY_LABELS["terraform"] },
  { value: "aws", label: CATEGORY_LABELS["aws"] },
];

const TRACK_PILLS: { value: Track; label: string }[] = [
  { value: "all", label: "All" },
  { value: "workloads", label: TRACK_LABELS["workloads"] },
  { value: "troubleshooting", label: TRACK_LABELS["troubleshooting"] },
  { value: "networking", label: TRACK_LABELS["networking"] },
  { value: "storage", label: TRACK_LABELS["storage"] },
  { value: "pod-security", label: TRACK_LABELS["pod-security"] },
  { value: "runtime-security", label: TRACK_LABELS["runtime-security"] },
  { value: "release-ops", label: TRACK_LABELS["release-ops"] },
  { value: "platform-eng", label: TRACK_LABELS["platform-eng"] },
];

export function Run() {
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState<Category>("all");
  const [track, setTrack] = useState<Track>("all");
  const { models, loading: modelsLoading } = useModels();
  const [selectedModel, setSelectedModel] = useState("");
  const { mode, setMode } = useEvidenceMode();
  const [copied, setCopied] = useState(false);

  const filtered = useMemo(() => {
    let list = SCENARIOS;
    if (category !== "all") {
      list = list.filter((s) => s.category === category);
    }
    if (track !== "all") {
      list = list.filter((s) => s.track === track);
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
  }, [category, track, search]);

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

  const { request } = useBenchApi();
  const [trigger, setTrigger] = useState<TriggerState>({ phase: "idle" });
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Clean up polling on unmount.
  useEffect(() => {
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, []);

  useEffect(() => {
    if (models.length === 0) {
      return;
    }
    if (!selectedModel || !models.some((model) => model.id === selectedModel)) {
      setSelectedModel(models[0].id);
    }
  }, [models, selectedModel]);

  const handleRun = useCallback(async () => {
    if (selectedIds.size === 0 || !selectedModel) return;
    setTrigger({ phase: "triggering" });

    try {
      const res = await request<{ id: string; status: string; total: string }>("/v1/bench/trigger", {
        method: "POST",
        body: JSON.stringify({
          model: selectedModel,
          evidence_mode: mode === "mcp" ? "mcp" : "none",
          scenarios: [...selectedIds],
        }),
      });

      const jobId = res.id;
      const total = parseInt(res.total, 10) || selectedIds.size;
      setTrigger({ phase: "running", jobId, completed: 0, total, status: "pending" });

      // Poll for progress.
      pollRef.current = setInterval(async () => {
        try {
          const job = await request<{ status: string; completed: number; total: number }>(
            `/v1/bench/trigger/${jobId}`
          );
          if (job.status === "completed" || job.status === "failed") {
            if (pollRef.current) clearInterval(pollRef.current);
            pollRef.current = null;
            setTrigger({ phase: "done", jobId, status: job.status, completed: job.completed, total: job.total });
          } else {
            setTrigger({ phase: "running", jobId, completed: job.completed, total: job.total, status: job.status });
          }
        } catch {
          // Ignore transient poll errors.
        }
      }, 3000);
    } catch (err: any) {
      setTrigger({ phase: "error", message: err.message || "Failed to trigger run" });
    }
  }, [selectedIds, selectedModel, request]);

  const command = useMemo(() => {
    if (selectedIds.size === 0 || !selectedModel) return null;
    return buildBenchCommand({
      scenarios: [...selectedIds],
      model: selectedModel,
      evidenceMode: mode === "mcp" ? "mcp" : "baseline",
    });
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
        <div className="glass-card overflow-hidden flex flex-col">
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

            {/* Track pills */}
            <div className="flex gap-1.5 flex-wrap">
              {TRACK_PILLS.map((pill) => (
                <button
                  key={pill.value}
                  onClick={() => setTrack(pill.value)}
                  className={`px-2.5 py-0.5 rounded-full text-[0.72rem] font-medium border transition-all ${
                    track === pill.value
                      ? "border-accent bg-accent/10 text-accent"
                      : "border-border-subtle text-fg-muted hover:border-accent/40 hover:text-fg"
                  }`}
                >
                  {pill.label}
                </button>
              ))}
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
                    <span
                      className={`px-1.5 py-0.5 rounded text-[0.65rem] font-medium ${LEVEL_COLORS[s.level]}`}
                    >
                      {s.level}
                    </span>
                    <span className="text-[0.6rem] text-fg-muted/60">
                      {TRACK_LABELS[s.track]}
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
          <div className="glass-card p-4">
            <label className="text-[0.72rem] font-semibold uppercase tracking-wider text-fg-muted mb-2.5 block">
              Model
            </label>
            {modelsLoading ? (
              <div className="text-[0.78rem] text-fg-muted">Loading models...</div>
            ) : models.length === 0 ? (
              <div className="text-[0.78rem] text-fg-muted">No models configured.</div>
            ) : (
              <div className="grid grid-cols-2 gap-2">
                {models.map((m) => (
                  <button
                    key={m.id}
                    onClick={() => setSelectedModel(m.id)}
                    className={`text-left px-3 py-2 rounded-md border text-[0.78rem] transition-all ${
                      selectedModel === m.id
                        ? "border-accent bg-accent/10 text-fg"
                        : "border-border text-fg-muted hover:border-accent/50"
                    }`}
                  >
                    <div className="font-medium">{m.display_name}</div>
                    {m.input_cost_per_mtok > 0 ? (
                      <div className="text-[0.68rem] text-fg-muted">
                        ${m.input_cost_per_mtok}/${m.output_cost_per_mtok} / MTok
                      </div>
                    ) : (
                      <div className="text-[0.68rem] text-fg-muted">Bundled fallback</div>
                    )}
                  </button>
                ))}
              </div>
            )}
          </div>

          {/* Evidence mode */}
          <div className="glass-card p-4">
            <label className="text-[0.72rem] font-semibold uppercase tracking-wider text-fg-muted mb-2.5 block">
              Evidence Mode
            </label>
            <div className="flex gap-2">
              {EVIDENCE_MODES.map((evidenceMode) => {
                const isSelected =
                  (mode === "none" && evidenceMode.id === "baseline") ||
                  (mode === "mcp" && evidenceMode.id === "mcp");
                return (
                  <button
                    key={evidenceMode.id}
                    onClick={() => setMode(evidenceMode.id === "baseline" ? "none" : "mcp")}
                    className={`flex-1 px-3 py-2 rounded-md border text-[0.78rem] font-medium transition-all ${
                      isSelected
                        ? "border-accent bg-accent/10 text-fg"
                        : "border-border text-fg-muted hover:border-accent/50"
                    }`}
                  >
                    {evidenceMode.label}
                    <span className="block text-[0.65rem] font-normal text-fg-muted">
                      {evidenceMode.description}
                    </span>
                  </button>
                );
              })}
            </div>
          </div>
        </div>
      </div>

      {/* Section 3: Run */}
      <div className="glass-card overflow-hidden">
        <div className="flex items-center justify-between px-4 py-3 border-b border-border-subtle">
          <label className="text-[0.72rem] font-semibold uppercase tracking-wider text-fg-muted">
            Run
          </label>
          <div className="flex items-center gap-2">
            {command && (
              <button
                onClick={handleCopy}
                className="inline-flex items-center gap-1.5 px-3 py-1 border border-border-subtle text-fg-muted text-[0.72rem] font-semibold rounded-md hover:border-accent/40 hover:text-fg transition-all"
              >
                {copied ? "Copied" : "Copy CLI"}
              </button>
            )}
            <button
              onClick={handleRun}
              disabled={selectedIds.size === 0 || trigger.phase === "triggering" || trigger.phase === "running"}
              className="inline-flex items-center gap-1.5 px-4 py-1 bg-accent text-white text-[0.72rem] font-semibold rounded-md hover:bg-accent/80 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {trigger.phase === "triggering" ? "Starting..." :
               trigger.phase === "running" ? `Running ${trigger.completed}/${trigger.total}` :
               "Run on Server"}
            </button>
          </div>
        </div>
        <div className="px-4 py-3">
          {trigger.phase === "running" && (
            <div className="space-y-2 mb-3">
              <div className="flex items-center justify-between text-[0.75rem]">
                <span className="text-fg-muted">Progress</span>
                <span className="text-fg font-medium">{trigger.completed} / {trigger.total}</span>
              </div>
              <div className="w-full bg-border-subtle rounded-full h-1.5">
                <div
                  className="bg-accent h-1.5 rounded-full transition-all duration-500"
                  style={{ width: `${trigger.total > 0 ? (trigger.completed / trigger.total) * 100 : 0}%` }}
                />
              </div>
            </div>
          )}
          {trigger.phase === "done" && (
            <div className={`flex items-center justify-between px-3 py-2 rounded-md mb-3 text-[0.78rem] font-medium ${
              trigger.status === "completed"
                ? "bg-green-500/10 text-green-400 border border-green-500/20"
                : "bg-red-500/10 text-red-400 border border-red-500/20"
            }`}>
              <span>{trigger.status === "completed" ? "Run completed" : "Run failed"} — {trigger.completed}/{trigger.total} scenarios</span>
              <a href="/bench" className="text-accent hover:text-accent/80 transition-colors text-[0.72rem]">
                View results
              </a>
            </div>
          )}
          {trigger.phase === "error" && (
            <div className="px-3 py-2 rounded-md mb-3 text-[0.78rem] bg-red-500/10 text-red-400 border border-red-500/20">
              {trigger.message}
            </div>
          )}
          {command ? (
            <details className="text-[0.72rem] text-fg-muted">
              <summary className="cursor-pointer hover:text-fg transition-colors">CLI command</summary>
              <pre className="mt-2 text-[0.75rem] text-fg font-mono leading-relaxed whitespace-pre overflow-x-auto">
                {command}
              </pre>
            </details>
          ) : (
            <p className="text-[0.82rem] text-fg-muted py-4 text-center">
              Select at least one scenario to run.
            </p>
          )}
        </div>
      </div>
    </div>
  );
}
