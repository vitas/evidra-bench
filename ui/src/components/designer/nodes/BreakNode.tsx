import { Handle, Position, type NodeProps, type Node } from "@xyflow/react";

export interface BreakData {
  [key: string]: unknown;
  kind: "break";
  method: "kubectl-apply" | "kubectl-patch" | "script";
  action: "wrong-image" | "missing-configmap" | "missing-secret" | "wrong-selector" | "wrong-probes" | "custom";
  target: string;
  customManifest: string;
}

type BreakNode = Node<BreakData, "break">;

const ACTION_LABELS: Record<string, string> = {
  "wrong-image": "Wrong image tag",
  "missing-configmap": "Missing ConfigMap",
  "missing-secret": "Missing Secret",
  "wrong-selector": "Wrong selector",
  "wrong-probes": "Wrong probes",
  custom: "Custom manifest",
};

export function BreakNode({ data, selected }: NodeProps<BreakNode>) {
  return (
    <div
      className={`min-w-[180px] rounded-lg border-l-4 border-l-red-500 bg-bg-elevated shadow-[var(--shadow-card)] transition-shadow ${
        selected ? "ring-2 ring-accent shadow-[var(--shadow-card-lg)]" : ""
      }`}
      style={{ border: selected ? undefined : "1px solid var(--color-border)" }}
    >
      <Handle
        type="target"
        position={Position.Left}
        className="!w-2.5 !h-2.5 !bg-red-500 !border-2 !border-bg-elevated"
      />
      <div className="px-3 py-2.5">
        <div className="flex items-center gap-2 mb-1">
          <span className="text-base">&#128165;</span>
          <span className="text-[0.82rem] font-semibold text-fg">Break</span>
        </div>
        <div className="text-[0.72rem] text-fg-muted">
          {ACTION_LABELS[data.action] ?? data.action}
        </div>
        {data.target && (
          <div className="text-[0.68rem] text-fg-muted mt-0.5 font-mono">
            {data.target}
          </div>
        )}
      </div>
      <Handle
        type="source"
        position={Position.Right}
        className="!w-2.5 !h-2.5 !bg-red-500 !border-2 !border-bg-elevated"
      />
    </div>
  );
}
