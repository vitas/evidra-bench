import { useState, useCallback } from "react";
import type { Node, Edge } from "@xyflow/react";
import { generateScenario, type PuzzleMetadata } from "./yaml-generator";
import { MODELS } from "../../data/models";
import { buildRunCommand, TOOL_BACKENDS, type ToolBackendId } from "../../lib/commandBuilder.mts";

interface RunButtonProps {
  metadata: PuzzleMetadata;
  nodes: Node[];
  edges: Edge[];
}

export function RunButton({ metadata, nodes, edges }: RunButtonProps) {
  const [open, setOpen] = useState(false);
  const [selectedModel, setSelectedModel] = useState(MODELS[0].id);
  const [toolBackend, setToolBackend] = useState<ToolBackendId>("baseline");
  const [result, setResult] = useState<{ command: string } | null>(null);

  const handleRun = useCallback(() => {
    const scenario = generateScenario(nodes, edges, metadata);
    if (scenario.warnings.length > 0 && !scenario.scenarioYaml) return;

    const command = buildRunCommand({
      scenario: `./${metadata.name || "my-puzzle"}`,
      model: selectedModel,
      toolBackend,
    });

    setResult({ command });
  }, [nodes, edges, metadata, selectedModel, toolBackend]);

  return (
    <>
      <button
        onClick={() => setOpen(!open)}
        className="inline-flex items-center gap-1 text-[0.72rem] font-medium text-accent hover:text-fg transition-colors"
      >
        <svg
          className="w-3 h-3"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth={2.5}
        >
          <polygon points="5 3 19 12 5 21 5 3" fill="currentColor" />
        </svg>
        Run
      </button>

      {open && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-bg-elevated border border-border rounded-xl shadow-2xl w-[480px] flex flex-col">
            {/* Header */}
            <div className="flex items-center justify-between px-5 py-3 border-b border-border-subtle">
              <h2 className="text-[0.92rem] font-bold text-fg">
                Run Puzzle
              </h2>
              <button
                onClick={() => { setOpen(false); setResult(null); }}
                className="text-fg-muted hover:text-fg text-lg transition-colors"
              >
                ✕
              </button>
            </div>

            {/* Content */}
            <div className="px-5 py-4 space-y-4">
              {/* Model selector */}
              <div>
                <label className="text-[0.72rem] font-semibold uppercase tracking-wider text-fg-muted mb-1.5 block">
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

              {/* Tool backend */}
              <div>
                <label className="text-[0.72rem] font-semibold uppercase tracking-wider text-fg-muted mb-1.5 block">
                  Tool Backend
                </label>
                <div className="flex gap-2">
                  {TOOL_BACKENDS.map((backend) => {
                    const isSelected = toolBackend === backend.id;
                    return (
                      <button
                        key={backend.id}
                        onClick={() => setToolBackend(backend.id)}
                        className={`flex-1 px-3 py-1.5 rounded-md border text-[0.78rem] font-medium transition-all ${
                          isSelected
                            ? "border-accent bg-accent/10 text-fg"
                            : "border-border text-fg-muted hover:border-accent/50"
                        }`}
                      >
                        {backend.label}
                        <span className="block text-[0.65rem] font-normal text-fg-muted">
                          {backend.description}
                        </span>
                      </button>
                    );
                  })}
                </div>
              </div>

              {/* Result / Command */}
              {result && (
                <div>
                  <label className="text-[0.72rem] font-semibold uppercase tracking-wider text-fg-muted mb-1.5 block">
                    Run Command
                  </label>
                  <pre className="bg-bg text-[0.72rem] text-fg-muted p-3 rounded-md border border-border overflow-x-auto font-mono leading-relaxed">
                    {result.command}
                  </pre>
                  <p className="text-[0.65rem] text-fg-muted mt-2">
                    Copy this command to run locally. Remote execution coming soon.
                  </p>
                </div>
              )}
            </div>

            {/* Footer */}
            <div className="flex justify-end gap-2 px-5 py-3 border-t border-border-subtle">
              {!result ? (
                <button
                  onClick={handleRun}
                  className="inline-flex items-center gap-1.5 px-4 py-1.5 bg-accent text-white text-[0.78rem] font-semibold rounded-md hover:bg-accent/80 transition-all"
                >
                  <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="currentColor">
                    <polygon points="5 3 19 12 5 21 5 3" />
                  </svg>
                  Generate Command
                </button>
              ) : (
                <button
                  onClick={() => {
                    navigator.clipboard.writeText(result.command);
                  }}
                  className="px-4 py-1.5 bg-accent text-white text-[0.78rem] font-semibold rounded-md hover:bg-accent/80 transition-all"
                >
                  Copy Command
                </button>
              )}
            </div>
          </div>
        </div>
      )}
    </>
  );
}
