import { Handle, Position, type NodeProps, type Node } from "@xyflow/react";

export interface TrapData {
  [key: string]: unknown;
  kind: "trap";
  trapName: string;
  detection: "resource-deleted" | "command-executed" | "custom";
  target: string;
}

type TrapNode = Node<TrapData, "trap">;

const DETECTION_LABELS: Record<string, string> = {
  "resource-deleted": "Resource Deleted",
  "command-executed": "Command Executed",
  custom: "Custom Rule",
};

export function TrapNode({ data, selected }: NodeProps<TrapNode>) {
  return (
    <div
      className={`min-w-[180px] rounded-lg border-l-4 border-l-amber-500 bg-bg-elevated shadow-[var(--shadow-card)] transition-shadow ${
        selected ? "ring-2 ring-accent shadow-[var(--shadow-card-lg)]" : ""
      }`}
      style={{ border: selected ? undefined : "1px solid var(--color-border)" }}
    >
      <Handle
        type="target"
        position={Position.Left}
        className="!w-2.5 !h-2.5 !bg-amber-500 !border-2 !border-bg-elevated"
      />
      <div className="px-3 py-2.5">
        <div className="flex items-center gap-2 mb-1">
          <span className="text-base">&#9888;&#65039;</span>
          <span className="text-[0.82rem] font-semibold text-fg">Trap</span>
        </div>
        <div className="text-[0.72rem] text-fg-muted">
          {data.trapName || "Unnamed trap"}
        </div>
        <div className="text-[0.68rem] text-fg-muted mt-0.5">
          {DETECTION_LABELS[data.detection] ?? data.detection}
        </div>
      </div>
    </div>
  );
}
