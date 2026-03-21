import { Handle, Position, type NodeProps, type Node } from "@xyflow/react";

export interface StackData {
  [key: string]: unknown;
  kind: "stack";
  stackType: "web-app" | "web-app-with-config" | "helm-app" | "custom";
  namespace: string;
}

type StackNode = Node<StackData, "stack">;

const STACK_LABELS: Record<string, string> = {
  "web-app": "Web App (nginx)",
  "web-app-with-config": "Web App + ConfigMap",
  "helm-app": "Helm Chart",
  custom: "Custom Stack",
};

export function StackNode({ data, selected }: NodeProps<StackNode>) {
  return (
    <div
      className={`min-w-[180px] rounded-lg border-l-4 border-l-blue-500 bg-bg-elevated shadow-[var(--shadow-card)] transition-shadow ${
        selected ? "ring-2 ring-accent shadow-[var(--shadow-card-lg)]" : ""
      }`}
      style={{ border: selected ? undefined : "1px solid var(--color-border)" }}
    >
      <div className="px-3 py-2.5">
        <div className="flex items-center gap-2 mb-1">
          <span className="text-base">&#128230;</span>
          <span className="text-[0.82rem] font-semibold text-fg">Stack</span>
        </div>
        <div className="text-[0.72rem] text-fg-muted">
          {STACK_LABELS[data.stackType] ?? data.stackType}
        </div>
        <div className="text-[0.68rem] text-fg-muted mt-0.5 font-mono">
          ns: {data.namespace}
        </div>
      </div>
      <Handle
        type="source"
        position={Position.Right}
        className="!w-2.5 !h-2.5 !bg-blue-500 !border-2 !border-bg-elevated"
      />
    </div>
  );
}
