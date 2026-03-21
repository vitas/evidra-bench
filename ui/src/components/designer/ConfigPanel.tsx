import { useCallback } from "react";
import type { Node } from "@xyflow/react";
import type { StackData } from "./nodes/StackNode";
import type { BreakData } from "./nodes/BreakNode";
import type { VerifyData } from "./nodes/VerifyNode";
import type { TrapData } from "./nodes/TrapNode";
import type { PuzzleMetadata, NodeData } from "./yaml-generator";

interface ConfigPanelProps {
  selectedNode: Node | null;
  metadata: PuzzleMetadata;
  onMetadataChange: (meta: PuzzleMetadata) => void;
  onNodeDataChange: (nodeId: string, data: Partial<NodeData>) => void;
  collapsed: boolean;
  onToggle: () => void;
}

function Label({ children }: { children: React.ReactNode }) {
  return (
    <label className="block text-[0.72rem] font-semibold text-fg-muted uppercase tracking-wider mb-1">
      {children}
    </label>
  );
}

function Input({
  value,
  onChange,
  placeholder,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
}) {
  return (
    <input
      type="text"
      value={value}
      onChange={(e) => onChange(e.target.value)}
      placeholder={placeholder}
      className="w-full px-2.5 py-1.5 text-[0.8rem] rounded-md border border-border bg-bg text-fg placeholder-fg-muted/50 focus:outline-none focus:border-accent transition-colors font-mono"
    />
  );
}

function Select({
  value,
  onChange,
  options,
}: {
  value: string;
  onChange: (v: string) => void;
  options: { value: string; label: string }[];
}) {
  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className="w-full px-2.5 py-1.5 text-[0.8rem] rounded-md border border-border bg-bg text-fg focus:outline-none focus:border-accent transition-colors"
    >
      {options.map((o) => (
        <option key={o.value} value={o.value}>
          {o.label}
        </option>
      ))}
    </select>
  );
}

function TextArea({
  value,
  onChange,
  placeholder,
  rows = 3,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  rows?: number;
}) {
  return (
    <textarea
      value={value}
      onChange={(e) => onChange(e.target.value)}
      placeholder={placeholder}
      rows={rows}
      className="w-full px-2.5 py-1.5 text-[0.8rem] rounded-md border border-border bg-bg text-fg placeholder-fg-muted/50 focus:outline-none focus:border-accent transition-colors font-mono resize-y"
    />
  );
}

function StackConfig({
  data,
  nodeId,
  onNodeDataChange,
}: {
  data: StackData;
  nodeId: string;
  onNodeDataChange: (id: string, d: Partial<NodeData>) => void;
}) {
  return (
    <>
      <SectionHeader icon="📦" label="Stack" color="text-blue-500" />
      <Field label="Stack Type">
        <Select
          value={data.stackType}
          onChange={(v) =>
            onNodeDataChange(nodeId, { stackType: v } as Partial<StackData>)
          }
          options={[
            { value: "web-app", label: "Web App (nginx)" },
            { value: "web-app-with-config", label: "Web App + ConfigMap" },
            { value: "helm-app", label: "Helm Chart" },
            { value: "custom", label: "Custom Stack" },
          ]}
        />
      </Field>
      <Field label="Namespace">
        <Input
          value={data.namespace}
          onChange={(v) =>
            onNodeDataChange(nodeId, { namespace: v } as Partial<StackData>)
          }
          placeholder="bench"
        />
      </Field>
    </>
  );
}

function BreakConfig({
  data,
  nodeId,
  onNodeDataChange,
}: {
  data: BreakData;
  nodeId: string;
  onNodeDataChange: (id: string, d: Partial<NodeData>) => void;
}) {
  return (
    <>
      <SectionHeader icon="💥" label="Break" color="text-red-500" />
      <Field label="Method">
        <Select
          value={data.method}
          onChange={(v) =>
            onNodeDataChange(nodeId, { method: v } as Partial<BreakData>)
          }
          options={[
            { value: "kubectl-apply", label: "kubectl apply" },
            { value: "kubectl-patch", label: "kubectl patch" },
            { value: "script", label: "Script" },
          ]}
        />
      </Field>
      <Field label="Action">
        <Select
          value={data.action}
          onChange={(v) =>
            onNodeDataChange(nodeId, { action: v } as Partial<BreakData>)
          }
          options={[
            { value: "wrong-image", label: "Wrong image tag" },
            { value: "missing-configmap", label: "Missing ConfigMap" },
            { value: "missing-secret", label: "Missing Secret" },
            { value: "wrong-selector", label: "Wrong selector" },
            { value: "wrong-probes", label: "Wrong probes" },
            { value: "custom", label: "Custom" },
          ]}
        />
      </Field>
      <Field label="Target">
        <Input
          value={data.target}
          onChange={(v) =>
            onNodeDataChange(nodeId, { target: v } as Partial<BreakData>)
          }
          placeholder="deployment/web"
        />
      </Field>
      {data.action === "custom" && (
        <Field label="Paste K8s Manifest">
          <TextArea
            value={data.customManifest}
            onChange={(v) =>
              onNodeDataChange(nodeId, {
                customManifest: v,
              } as Partial<BreakData>)
            }
            placeholder={"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n  namespace: bench\nspec:\n  replicas: 1\n  ..."}
            rows={12}
          />
        </Field>
      )}
    </>
  );
}

function VerifyConfig({
  data,
  nodeId,
  onNodeDataChange,
}: {
  data: VerifyData;
  nodeId: string;
  onNodeDataChange: (id: string, d: Partial<NodeData>) => void;
}) {
  return (
    <>
      <SectionHeader icon="✓" label="Verify" color="text-emerald-500" />
      <Field label="Check Type">
        <Select
          value={data.checkType}
          onChange={(v) =>
            onNodeDataChange(nodeId, { checkType: v } as Partial<VerifyData>)
          }
          options={[
            { value: "deployment-ready", label: "Deployment Ready" },
            { value: "service-endpoints", label: "Service Endpoints" },
            { value: "resource-exists", label: "Resource Exists" },
            { value: "helm-release", label: "Helm Release" },
          ]}
        />
      </Field>
      <Field label="Namespace">
        <Input
          value={data.namespace}
          onChange={(v) =>
            onNodeDataChange(nodeId, { namespace: v } as Partial<VerifyData>)
          }
          placeholder="bench"
        />
      </Field>
      <Field label="Resource Name">
        <Input
          value={data.resourceName}
          onChange={(v) =>
            onNodeDataChange(nodeId, {
              resourceName: v,
            } as Partial<VerifyData>)
          }
          placeholder="web"
        />
      </Field>
    </>
  );
}

function TrapConfig({
  data,
  nodeId,
  onNodeDataChange,
}: {
  data: TrapData;
  nodeId: string;
  onNodeDataChange: (id: string, d: Partial<NodeData>) => void;
}) {
  return (
    <>
      <SectionHeader icon="⚠️" label="Trap" color="text-amber-500" />
      <Field label="Trap Name">
        <Input
          value={data.trapName}
          onChange={(v) =>
            onNodeDataChange(nodeId, { trapName: v } as Partial<TrapData>)
          }
          placeholder="e.g. resource_deleted"
        />
      </Field>
      <Field label="Detection">
        <Select
          value={data.detection}
          onChange={(v) =>
            onNodeDataChange(nodeId, { detection: v } as Partial<TrapData>)
          }
          options={[
            { value: "resource-deleted", label: "Resource Deleted" },
            { value: "command-executed", label: "Command Executed" },
            { value: "custom", label: "Custom Rule" },
          ]}
        />
      </Field>
      <Field label="Target">
        <Input
          value={data.target}
          onChange={(v) =>
            onNodeDataChange(nodeId, { target: v } as Partial<TrapData>)
          }
          placeholder="deployment/web"
        />
      </Field>
    </>
  );
}

function MetadataConfig({
  metadata,
  onMetadataChange,
}: {
  metadata: PuzzleMetadata;
  onMetadataChange: (m: PuzzleMetadata) => void;
}) {
  return (
    <>
      <SectionHeader icon="🧩" label="Puzzle Metadata" color="text-accent" />
      <Field label="Puzzle Name *">
        <Input
          value={metadata.name}
          onChange={(v) => onMetadataChange({ ...metadata, name: v })}
          placeholder="broken-deployment"
        />
      </Field>
      <Field label="Title *">
        <Input
          value={metadata.title}
          onChange={(v) => onMetadataChange({ ...metadata, title: v })}
          placeholder="Fix a broken deployment"
        />
      </Field>
      <Field label="Description">
        <TextArea
          value={metadata.description}
          onChange={(v) => onMetadataChange({ ...metadata, description: v })}
          placeholder="Describe the scenario..."
          rows={4}
        />
      </Field>
      <Field label="Difficulty">
        <Select
          value={metadata.difficulty}
          onChange={(v) =>
            onMetadataChange({
              ...metadata,
              difficulty: v as PuzzleMetadata["difficulty"],
            })
          }
          options={[
            { value: "easy", label: "Easy" },
            { value: "medium", label: "Medium" },
            { value: "hard", label: "Hard" },
          ]}
        />
      </Field>
      <Field label="Time Limit">
        <Input
          value={metadata.timeLimit}
          onChange={(v) => onMetadataChange({ ...metadata, timeLimit: v })}
          placeholder="5m"
        />
      </Field>
      <Field label="Category">
        <Select
          value={metadata.category}
          onChange={(v) =>
            onMetadataChange({
              ...metadata,
              category: v as PuzzleMetadata["category"],
            })
          }
          options={[
            { value: "kubernetes", label: "Kubernetes" },
            { value: "helm", label: "Helm" },
            { value: "argocd", label: "Argo CD" },
            { value: "terraform", label: "Terraform" },
          ]}
        />
      </Field>
    </>
  );
}

function SectionHeader({
  icon,
  label,
  color,
}: {
  icon: string;
  label: string;
  color: string;
}) {
  return (
    <div className="flex items-center gap-2 mb-3 pb-2 border-b border-border-subtle">
      <span className={`text-sm ${color}`}>{icon}</span>
      <span className="text-[0.82rem] font-bold text-fg">{label}</span>
    </div>
  );
}

function Field({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="mb-3">
      <Label>{label}</Label>
      {children}
    </div>
  );
}

export function ConfigPanel({
  selectedNode,
  metadata,
  onMetadataChange,
  onNodeDataChange,
  collapsed,
  onToggle,
}: ConfigPanelProps) {
  const renderConfig = useCallback(() => {
    if (!selectedNode) {
      return (
        <MetadataConfig
          metadata={metadata}
          onMetadataChange={onMetadataChange}
        />
      );
    }

    const data = selectedNode.data as NodeData;
    const nodeId = selectedNode.id;

    switch (data.kind) {
      case "stack":
        return (
          <StackConfig
            data={data}
            nodeId={nodeId}
            onNodeDataChange={onNodeDataChange}
          />
        );
      case "break":
        return (
          <BreakConfig
            data={data}
            nodeId={nodeId}
            onNodeDataChange={onNodeDataChange}
          />
        );
      case "verify":
        return (
          <VerifyConfig
            data={data}
            nodeId={nodeId}
            onNodeDataChange={onNodeDataChange}
          />
        );
      case "trap":
        return (
          <TrapConfig
            data={data}
            nodeId={nodeId}
            onNodeDataChange={onNodeDataChange}
          />
        );
      default:
        return null;
    }
  }, [selectedNode, metadata, onMetadataChange, onNodeDataChange]);

  return (
    <div
      className={`shrink-0 border-l border-border-subtle bg-bg-alt overflow-y-auto transition-all ${
        collapsed ? "w-0 overflow-hidden" : "w-[300px] max-md:w-[240px]"
      }`}
    >
      <div className="px-4 py-3">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-[0.72rem] font-bold text-fg-muted uppercase tracking-wider">
            {selectedNode ? "Block Config" : "Puzzle Config"}
          </h3>
          <button
            onClick={onToggle}
            className="text-fg-muted hover:text-fg text-[0.8rem] transition-colors"
            title="Collapse panel"
          >
            &#x2715;
          </button>
        </div>
        {renderConfig()}
      </div>
    </div>
  );
}
