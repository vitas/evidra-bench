import { Handle, Position, type NodeProps, type Node } from "@xyflow/react";

export interface InfraData {
  [key: string]: unknown;
  kind: "infra";
  cni: "" | "cilium" | "calico";
  addons: string[];
  runtimes: string[];
  features: string[];
}

type InfraNode = Node<InfraData, "infra">;

function formatList(items: string[]): string {
  if (items.length === 0) return "none";
  return items.join(", ");
}

export function InfraNode({ data, selected }: NodeProps<InfraNode>) {
  const hasCNI = data.cni !== "";
  const hasAddons = data.addons.length > 0;
  const hasRuntimes = data.runtimes.length > 0;
  const hasFeatures = data.features.length > 0;
  const isEmpty = !hasCNI && !hasAddons && !hasRuntimes && !hasFeatures;

  return (
    <div
      className={`min-w-[180px] rounded-lg border-l-4 border-l-purple-500 bg-bg-elevated shadow-[var(--shadow-card)] transition-shadow ${
        selected ? "ring-2 ring-accent shadow-[var(--shadow-card-lg)]" : ""
      }`}
      style={{ border: selected ? undefined : "1px solid var(--color-border)" }}
    >
      <div className="px-3 py-2.5">
        <div className="flex items-center gap-2 mb-1">
          <span className="text-base">&#9881;&#65039;</span>
          <span className="text-[0.82rem] font-semibold text-fg">Infra</span>
        </div>
        {isEmpty ? (
          <div className="text-[0.72rem] text-fg-muted">Default cluster</div>
        ) : (
          <div className="text-[0.68rem] text-fg-muted space-y-0.5">
            {hasCNI && <div className="font-mono">cni: {data.cni}</div>}
            {hasAddons && <div className="font-mono">addons: {formatList(data.addons)}</div>}
            {hasRuntimes && <div className="font-mono">runtimes: {formatList(data.runtimes)}</div>}
            {hasFeatures && <div className="font-mono">features: {formatList(data.features)}</div>}
          </div>
        )}
      </div>
      <Handle
        type="source"
        position={Position.Right}
        className="!w-2.5 !h-2.5 !bg-purple-500 !border-2 !border-bg-elevated"
      />
    </div>
  );
}
