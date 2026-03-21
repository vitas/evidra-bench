import {
  useState,
  useCallback,
  useRef,
  useMemo,
  useEffect,
  type DragEvent,
} from "react";
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  addEdge,
  useNodesState,
  useEdgesState,
  type Connection,
  type Node,
  type Edge,
  type NodeTypes,
  BackgroundVariant,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";

import { Palette } from "../components/designer/Palette";
import { ConfigPanel } from "../components/designer/ConfigPanel";
import { ExportButton } from "../components/designer/ExportButton";
import { RunButton } from "../components/designer/RunButton";
import { GuidedTour, useTourState } from "../components/designer/GuidedTour";
import { TemplatesModal } from "../components/designer/TemplatesModal";
import { StackNode, type StackData } from "../components/designer/nodes/StackNode";
import { BreakNode, type BreakData } from "../components/designer/nodes/BreakNode";
import { VerifyNode, type VerifyData } from "../components/designer/nodes/VerifyNode";
import { TrapNode, type TrapData } from "../components/designer/nodes/TrapNode";
import type { PuzzleMetadata, NodeData } from "../components/designer/yaml-generator";

const nodeTypes: NodeTypes = {
  stack: StackNode,
  break: BreakNode,
  verify: VerifyNode,
  trap: TrapNode,
};

function makeDefaultData(kind: string): NodeData {
  switch (kind) {
    case "stack":
      return { kind: "stack", stackType: "web-app", namespace: "bench" } as StackData;
    case "break":
      return {
        kind: "break",
        method: "kubectl-apply",
        action: "wrong-image",
        target: "deployment/web",
        customManifest: "",
      } as BreakData;
    case "verify":
      return {
        kind: "verify",
        checkType: "deployment-ready",
        namespace: "bench",
        resourceName: "web",
      } as VerifyData;
    case "trap":
      return {
        kind: "trap",
        trapName: "",
        detection: "resource-deleted",
        target: "",
      } as TrapData;
    default:
      return { kind: "stack", stackType: "web-app", namespace: "bench" } as StackData;
  }
}

// Pre-populated example: Stack -> Break -> Verify
const EXAMPLE_NODES: Node[] = [
  {
    id: "stack-1",
    type: "stack",
    position: { x: 50, y: 120 },
    data: { kind: "stack", stackType: "web-app", namespace: "bench" },
  },
  {
    id: "break-1",
    type: "break",
    position: { x: 320, y: 120 },
    data: {
      kind: "break",
      method: "kubectl-apply",
      action: "wrong-image",
      target: "deployment/web",
      customManifest: "",
    },
  },
  {
    id: "verify-1",
    type: "verify",
    position: { x: 590, y: 120 },
    data: {
      kind: "verify",
      checkType: "deployment-ready",
      namespace: "bench",
      resourceName: "web",
    },
  },
];

const EDGE_STYLE = { stroke: "var(--color-accent)", strokeWidth: 2, opacity: 0.7 };

const EXAMPLE_EDGES: Edge[] = [
  {
    id: "e-stack-break",
    source: "stack-1",
    target: "break-1",
    animated: true,
    style: EDGE_STYLE,
  },
  {
    id: "e-break-verify",
    source: "break-1",
    target: "verify-1",
    animated: true,
    style: EDGE_STYLE,
  },
];

const DEFAULT_METADATA: PuzzleMetadata = {
  name: "my-puzzle",
  title: "Fix a broken deployment with bad image",
  description:
    "The deployment uses image tag nginx:99.99-nonexistent which doesn't exist.\nPods are stuck in ErrImagePull/ImagePullBackOff.",
  difficulty: "easy",
  timeLimit: "5m",
  category: "kubernetes",
};

const DRAFT_KEY = "designer-draft";

interface DraftState {
  nodes: Node[];
  edges: Edge[];
  metadata: PuzzleMetadata;
}

function loadDraft(): DraftState | null {
  try {
    const saved = localStorage.getItem(DRAFT_KEY);
    if (saved) {
      return JSON.parse(saved) as DraftState;
    }
  } catch {
    // Ignore corrupt data
  }
  return null;
}

let nodeIdCounter = 10;
function nextId(type: string): string {
  nodeIdCounter += 1;
  return `${type}-${nodeIdCounter}`;
}

export function Designer() {
  const reactFlowWrapper = useRef<HTMLDivElement>(null);

  const draft = useMemo(() => loadDraft(), []);

  const [nodes, setNodes, onNodesChange] = useNodesState(draft?.nodes ?? EXAMPLE_NODES);
  const [edges, setEdges, onEdgesChange] = useEdgesState(draft?.edges ?? EXAMPLE_EDGES);
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const [metadata, setMetadata] = useState<PuzzleMetadata>(draft?.metadata ?? DEFAULT_METADATA);
  const [panelCollapsed, setPanelCollapsed] = useState(false);
  const [minimapOpen, setMinimapOpen] = useState(false);
  const [templatesOpen, setTemplatesOpen] = useState(false);
  const [templatesMode, setTemplatesMode] = useState<"run" | "new">("run");
  const [draftSaved, setDraftSaved] = useState(false);
  const tour = useTourState();

  // Autosave to localStorage (debounced 300ms)
  useEffect(() => {
    const timer = setTimeout(() => {
      localStorage.setItem(DRAFT_KEY, JSON.stringify({ nodes, edges, metadata }));
      setDraftSaved(true);
      const fadeTimer = setTimeout(() => setDraftSaved(false), 1500);
      return () => clearTimeout(fadeTimer);
    }, 300);
    return () => clearTimeout(timer);
  }, [nodes, edges, metadata]);

  const selectedNode = useMemo(
    () => nodes.find((n) => n.id === selectedNodeId) ?? null,
    [nodes, selectedNodeId],
  );

  const onConnect = useCallback(
    (connection: Connection) => {
      setEdges((eds) =>
        addEdge(
          {
            ...connection,
            animated: true,
            style: EDGE_STYLE,
          },
          eds,
        ),
      );
    },
    [setEdges],
  );

  const onDragOver = useCallback((event: DragEvent) => {
    event.preventDefault();
    event.dataTransfer.dropEffect = "move";
  }, []);

  const onDrop = useCallback(
    (event: DragEvent) => {
      event.preventDefault();
      const type = event.dataTransfer.getData("application/reactflow");
      if (!type || !reactFlowWrapper.current) return;

      const bounds = reactFlowWrapper.current.getBoundingClientRect();
      const position = {
        x: event.clientX - bounds.left - 90,
        y: event.clientY - bounds.top - 30,
      };

      const newNode: Node = {
        id: nextId(type),
        type,
        position,
        data: makeDefaultData(type),
      };

      setNodes((nds) => [...nds, newNode]);
    },
    [setNodes],
  );

  const onNodeClick = useCallback(
    (_: React.MouseEvent, node: Node) => {
      setSelectedNodeId(node.id);
      if (panelCollapsed) setPanelCollapsed(false);
    },
    [panelCollapsed],
  );

  const onPaneClick = useCallback(() => {
    setSelectedNodeId(null);
  }, []);

  const onNodeDataChange = useCallback(
    (nodeId: string, partial: Partial<NodeData>) => {
      setNodes((nds) =>
        nds.map((n) => {
          if (n.id !== nodeId) return n;
          return { ...n, data: { ...n.data, ...partial } };
        }),
      );
    },
    [setNodes],
  );

  const handleClear = useCallback(() => {
    if (nodes.length > 0 && !window.confirm("Clear the canvas? This will remove all blocks.")) {
      return;
    }
    setNodes([]);
    setEdges([]);
    setSelectedNodeId(null);
    setMetadata(DEFAULT_METADATA);
    localStorage.removeItem(DRAFT_KEY);
  }, [nodes.length, setNodes, setEdges]);

  const handleTemplateSelect = useCallback(
    (template: { nodes: Node[]; edges: Edge[]; metadata: PuzzleMetadata }) => {
      if (nodes.length > 0 && !window.confirm("Load this template? It will replace your current canvas.")) {
        return;
      }
      setNodes(template.nodes);
      setEdges(template.edges);
      setMetadata(template.metadata);
      setSelectedNodeId(null);
      setTemplatesOpen(false);
    },
    [nodes.length, setNodes, setEdges],
  );

  return (
    <div className="flex relative" style={{ height: "calc(100vh - 110px)" }}>
      <GuidedTour
        nodes={nodes}
        edges={edges}
        active={tour.active}
        onComplete={tour.complete}
      />
      <Palette />

      <div ref={reactFlowWrapper} className="flex-1 flex flex-col relative" data-tour="canvas">
        {/* Toolbar */}
        <div className="flex items-center justify-between px-3 py-1.5 border-b border-border-subtle bg-bg-elevated/50">
          <div className="flex items-center gap-3">
            {!tour.active && (
              <button
                onClick={tour.restart}
                className="text-[0.72rem] font-medium text-fg-muted/60 hover:text-fg transition-colors"
                title="Restart guided tour"
              >
                ? Tour
              </button>
            )}
            <button
              onClick={() => setMinimapOpen(!minimapOpen)}
              className="text-[0.72rem] font-medium text-fg-muted/60 hover:text-fg transition-colors"
              title={minimapOpen ? "Hide minimap" : "Show minimap"}
            >
              {minimapOpen ? "Hide Map" : "Map"}
            </button>
            <span className="text-border">|</span>
            <button
              onClick={() => { setTemplatesOpen(true); setTemplatesMode("run"); }}
              className="text-[0.72rem] font-medium text-fg-muted hover:text-fg transition-colors"
              title="Browse scenarios and configure benchmark runs"
            >
              Scenarios
            </button>
            <button
              onClick={() => { setTemplatesOpen(true); setTemplatesMode("new"); }}
              className="text-[0.72rem] font-medium text-fg-muted hover:text-fg transition-colors"
              title="Create a new puzzle"
            >
              New Puzzle
            </button>
            <button
              onClick={handleClear}
              className="text-[0.72rem] font-medium text-fg-muted hover:text-fg transition-colors"
              title="Clear canvas"
            >
              Clear
            </button>
            <span
              className={`text-[0.65rem] text-fg-muted/60 transition-opacity duration-700 ${
                draftSaved ? "opacity-100" : "opacity-0"
              }`}
            >
              Draft saved
            </span>
          </div>
          <div className="flex items-center gap-3">
            <div data-tour="export-button">
              <ExportButton nodes={nodes} edges={edges} metadata={metadata} />
            </div>
            {panelCollapsed && (
              <button
                onClick={() => setPanelCollapsed(false)}
                className="text-[0.72rem] font-medium text-fg-muted hover:text-fg transition-colors"
              >
                Config
              </button>
            )}
            <RunButton metadata={metadata} nodes={nodes} edges={edges} />
          </div>
        </div>

        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          onConnect={onConnect}
          onDrop={onDrop}
          onDragOver={onDragOver}
          onNodeClick={onNodeClick}
          onPaneClick={onPaneClick}
          nodeTypes={nodeTypes}
          fitView
          snapToGrid
          snapGrid={[15, 15]}
          connectionLineStyle={{ stroke: "var(--color-accent)" }}
          defaultEdgeOptions={{
            animated: true,
            style: EDGE_STYLE,
          }}
          deleteKeyCode="Backspace"
        >
          <Background
            variant={BackgroundVariant.Dots}
            gap={20}
            size={1}
            color="var(--color-border)"
          />
          <Controls
            className="!bg-bg-elevated !border-border !shadow-[var(--shadow-card)]"
          />
          {minimapOpen && (
            <MiniMap
              className="!bg-bg-alt !border-border"
              nodeColor={() => "var(--color-accent)"}
              maskColor="rgba(0,0,0,0.15)"
            />
          )}
        </ReactFlow>

      </div>

      <div data-tour="config-panel">
        <ConfigPanel
          selectedNode={selectedNode}
          metadata={metadata}
          onMetadataChange={setMetadata}
          onNodeDataChange={onNodeDataChange}
          collapsed={panelCollapsed}
          onToggle={() => setPanelCollapsed(!panelCollapsed)}
        />
      </div>

      <TemplatesModal
        open={templatesOpen}
        onClose={() => setTemplatesOpen(false)}
        onSelect={handleTemplateSelect}
        initialMode={templatesMode}
      />
    </div>
  );
}
