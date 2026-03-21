import { Handle, Position, type NodeProps, type Node } from "@xyflow/react";

export interface VerifyData {
  [key: string]: unknown;
  kind: "verify";
  checkType: "deployment-ready" | "service-endpoints" | "resource-exists" | "helm-release";
  namespace: string;
  resourceName: string;
}

type VerifyNode = Node<VerifyData, "verify">;

const CHECK_LABELS: Record<string, string> = {
  "deployment-ready": "Deployment Ready",
  "service-endpoints": "Service Endpoints",
  "resource-exists": "Resource Exists",
  "helm-release": "Helm Release",
};

export function VerifyNode({ data, selected }: NodeProps<VerifyNode>) {
  return (
    <div
      className={`min-w-[180px] rounded-lg border-l-4 border-l-emerald-500 bg-bg-elevated shadow-[var(--shadow-card)] transition-shadow ${
        selected ? "ring-2 ring-accent shadow-[var(--shadow-card-lg)]" : ""
      }`}
      style={{ border: selected ? undefined : "1px solid var(--color-border)" }}
    >
      <Handle
        type="target"
        position={Position.Left}
        className="!w-2.5 !h-2.5 !bg-emerald-500 !border-2 !border-bg-elevated"
      />
      <div className="px-3 py-2.5">
        <div className="flex items-center gap-2 mb-1">
          <span className="text-base text-emerald-500 font-bold">&#10003;</span>
          <span className="text-[0.82rem] font-semibold text-fg">Verify</span>
        </div>
        <div className="text-[0.72rem] text-fg-muted">
          {CHECK_LABELS[data.checkType] ?? data.checkType}
        </div>
        {data.resourceName && (
          <div className="text-[0.68rem] text-fg-muted mt-0.5 font-mono">
            {data.namespace}/{data.resourceName}
          </div>
        )}
      </div>
    </div>
  );
}
