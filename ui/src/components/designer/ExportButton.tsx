import { useState, useCallback } from "react";
import type { Node, Edge } from "@xyflow/react";
import { generateScenario, type PuzzleMetadata } from "./yaml-generator";

interface ExportButtonProps {
  nodes: Node[];
  edges: Edge[];
  metadata: PuzzleMetadata;
}

type Tab = "scenario" | "prompt" | "fixture";

export function ExportButton({ nodes, edges, metadata }: ExportButtonProps) {
  const [open, setOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<Tab>("scenario");
  const [copied, setCopied] = useState(false);

  const result = open ? generateScenario(nodes, edges, metadata) : null;

  const handleCopy = useCallback(() => {
    if (!result) return;
    const text =
      activeTab === "scenario"
        ? result.scenarioYaml
        : activeTab === "prompt"
          ? result.taskPrompt
          : result.fixtureYaml;
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }, [result, activeTab]);

  const currentContent = result
    ? activeTab === "scenario"
      ? result.scenarioYaml
      : activeTab === "prompt"
        ? result.taskPrompt
        : result.fixtureYaml
    : "";

  const tabs: { id: Tab; label: string; file: string }[] = [
    { id: "scenario", label: "Scenario", file: "scenario.yaml" },
    { id: "prompt", label: "Prompt", file: "prompts/task.md" },
    { id: "fixture", label: "Fixture", file: "fixtures/broken.yaml" },
  ];

  return (
    <>
      <button
        onClick={() => setOpen(true)}
        className="inline-flex items-center gap-1 text-[0.72rem] font-medium text-fg-muted hover:text-fg transition-colors"
      >
        <svg
          className="w-3 h-3"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
          />
        </svg>
        Export YAML
      </button>

      {open && result && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-bg-elevated border border-border rounded-xl shadow-2xl w-[720px] max-h-[80vh] flex flex-col">
            {/* Header */}
            <div className="flex items-center justify-between px-5 py-3 border-b border-border-subtle">
              <h2 className="text-[0.92rem] font-bold text-fg">
                Generated Scenario
              </h2>
              <button
                onClick={() => setOpen(false)}
                className="text-fg-muted hover:text-fg text-lg transition-colors"
              >
                &#x2715;
              </button>
            </div>

            {/* Warnings */}
            {result.warnings.length > 0 && (
              <div className="mx-5 mt-3 px-3 py-2 rounded-md bg-amber-500/10 border border-amber-500/30 text-[0.78rem] text-amber-400">
                {result.warnings.map((w, i) => (
                  <div key={i} className="flex items-start gap-1.5">
                    <span className="shrink-0">&#9888;</span>
                    <span>{w}</span>
                  </div>
                ))}
              </div>
            )}

            {/* Tabs */}
            <div className="flex gap-1 px-5 pt-3">
              {tabs.map((tab) => (
                <button
                  key={tab.id}
                  onClick={() => setActiveTab(tab.id)}
                  className={`px-3 py-1.5 text-[0.78rem] font-medium rounded-t-md transition-colors ${
                    activeTab === tab.id
                      ? "bg-bg text-fg border border-border-subtle border-b-transparent"
                      : "text-fg-muted hover:text-fg"
                  }`}
                >
                  {tab.label}
                  <span className="ml-1.5 text-[0.68rem] text-fg-muted font-mono">
                    {tab.file}
                  </span>
                </button>
              ))}
            </div>

            {/* Content */}
            <div className="flex-1 overflow-auto mx-5 mb-4 rounded-b-md rounded-tr-md border border-border-subtle bg-bg">
              <pre className="p-4 text-[0.75rem] text-fg-body font-mono leading-relaxed whitespace-pre-wrap">
                {currentContent || "# (empty)"}
              </pre>
            </div>

            {/* Footer */}
            <div className="flex items-center justify-end gap-3 px-5 py-3 border-t border-border-subtle">
              <button
                onClick={handleCopy}
                className="inline-flex items-center gap-1.5 px-3.5 py-1.5 text-[0.8rem] font-medium border border-border rounded-md text-fg hover:border-accent hover:text-accent transition-colors"
              >
                {copied ? "Copied!" : "Copy"}
              </button>
              <button
                onClick={() => setOpen(false)}
                className="px-3.5 py-1.5 text-[0.8rem] font-medium bg-accent text-white rounded-md hover:bg-accent-bright transition-colors"
              >
                Done
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
