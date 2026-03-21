import type { Node, Edge } from "@xyflow/react";
import type { StackData } from "./nodes/StackNode";
import type { BreakData } from "./nodes/BreakNode";
import type { VerifyData } from "./nodes/VerifyNode";
import type { TrapData } from "./nodes/TrapNode";

export type NodeData = StackData | BreakData | VerifyData | TrapData;

export interface PuzzleMetadata {
  name: string;
  title: string;
  description: string;
  difficulty: "easy" | "medium" | "hard";
  timeLimit: string;
  category: "kubernetes" | "helm" | "argocd" | "terraform";
}

export interface GeneratedScenario {
  scenarioYaml: string;
  taskPrompt: string;
  fixtureYaml: string;
  warnings: string[];
}

interface BreakPreset {
  description: string;
  patch: Record<string, unknown> | null;
  fixtureYaml: string;
}

const BREAK_PRESETS: Record<string, BreakPreset> = {
  "wrong-image": {
    description: "Deployment uses nonexistent image tag",
    patch: {
      spec: {
        template: {
          spec: {
            containers: [{ name: "nginx", image: "nginx:99.99-nonexistent" }],
          },
        },
      },
    },
    fixtureYaml: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: bench
spec:
  replicas: 2
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      containers:
        - name: nginx
          image: nginx:99.99-nonexistent
          ports:
            - containerPort: 80`,
  },
  "missing-configmap": {
    description: "Deployment references a deleted ConfigMap",
    patch: null,
    fixtureYaml: `# This break deletes the ConfigMap that the deployment depends on.
# Apply this after baseline is running.
# kubectl delete configmap app-config -n bench`,
  },
  "missing-secret": {
    description: "Deployment references a missing Secret",
    patch: null,
    fixtureYaml: `# This break deletes the Secret that the deployment depends on.
# kubectl delete secret app-secret -n bench`,
  },
  "wrong-selector": {
    description: "Service selector does not match any pods",
    patch: null,
    fixtureYaml: `apiVersion: v1
kind: Service
metadata:
  name: web
  namespace: bench
spec:
  selector:
    app: web-WRONG
  ports:
    - port: 80
      targetPort: 80`,
  },
  "wrong-probes": {
    description: "Liveness probe points to a non-existent path",
    patch: null,
    fixtureYaml: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: bench
spec:
  replicas: 2
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      containers:
        - name: nginx
          image: nginx:1.27
          ports:
            - containerPort: 80
          livenessProbe:
            httpGet:
              path: /nonexistent-health
              port: 80
            initialDelaySeconds: 3
            periodSeconds: 5
            failureThreshold: 2`,
  },
};

function toYamlString(value: string): string {
  if (
    value.includes(":") ||
    value.includes("#") ||
    value.includes("'") ||
    value.includes('"') ||
    value.includes("\n")
  ) {
    return `"${value.replace(/"/g, '\\"')}"`;
  }
  return value;
}

// buildStages groups BreakNodes with their connected VerifyNodes and TrapNodes
// using edge relationships, ordered by x-position (left to right).
interface StageGroup {
  breakNode: Node;
  breakData: BreakData;
  verifyNodes: Node[];
  trapNodes: Node[];
}

function buildStageGroups(
  breakNodes: Node[],
  verifyNodes: Node[],
  trapNodes: Node[],
  edges: Edge[],
): StageGroup[] {
  // Build adjacency: source -> target[]
  const outgoing = new Map<string, Set<string>>();
  for (const e of edges) {
    if (!outgoing.has(e.source)) outgoing.set(e.source, new Set());
    outgoing.get(e.source)!.add(e.target);
  }

  const verifyById = new Map(verifyNodes.map((n) => [n.id, n]));
  const trapById = new Map(trapNodes.map((n) => [n.id, n]));

  const stages: StageGroup[] = breakNodes.map((bn) => {
    const targets = outgoing.get(bn.id) ?? new Set<string>();
    const connectedVerify: Node[] = [];
    const connectedTrap: Node[] = [];
    for (const tid of targets) {
      if (verifyById.has(tid)) connectedVerify.push(verifyById.get(tid)!);
      if (trapById.has(tid)) connectedTrap.push(trapById.get(tid)!);
    }
    // Also check if trap connects TO the break node (trap -> break edge)
    for (const tn of trapNodes) {
      const trapTargets = outgoing.get(tn.id) ?? new Set<string>();
      if (trapTargets.has(bn.id) && !connectedTrap.includes(tn)) {
        connectedTrap.push(tn);
      }
    }
    return {
      breakNode: bn,
      breakData: bn.data as BreakData,
      verifyNodes: connectedVerify,
      trapNodes: connectedTrap,
    };
  });

  // Sort by x-position (left to right)
  stages.sort((a, b) => (a.breakNode.position.x - b.breakNode.position.x));

  return stages;
}

function generateStageName(breakData: BreakData, index: number): string {
  if (breakData.action !== "custom") return breakData.action;
  return `stage-${index + 1}`;
}

export function generateScenario(
  nodes: Node[],
  edges: Edge[],
  metadata: PuzzleMetadata,
): GeneratedScenario {
  const warnings: string[] = [];

  const stackNodes = nodes.filter(
    (n) => (n.data as NodeData).kind === "stack",
  );
  const breakNodes = nodes.filter(
    (n) => (n.data as NodeData).kind === "break",
  );
  const verifyNodes = nodes.filter(
    (n) => (n.data as NodeData).kind === "verify",
  );
  const trapNodes = nodes.filter(
    (n) => (n.data as NodeData).kind === "trap",
  );

  if (stackNodes.length === 0) warnings.push("No Stack block: the puzzle has no baseline to deploy.");
  if (breakNodes.length === 0) warnings.push("No Break block: the puzzle has nothing to break.");
  if (verifyNodes.length === 0) warnings.push("No Verify block: the puzzle has no success criteria.");

  if (!metadata.name.trim()) warnings.push("Puzzle name is required.");
  if (!metadata.title.trim()) warnings.push("Puzzle title is required.");

  const id = metadata.name.trim().toLowerCase().replace(/\s+/g, "-").replace(/[^a-z0-9-]/g, "");

  const isMultiStage = breakNodes.length > 1;

  // Validate linear chain for multi-stage.
  if (isMultiStage) {
    const stageGroups = buildStageGroups(breakNodes, verifyNodes, trapNodes, edges);
    // Each break must connect to at least one verify.
    for (const sg of stageGroups) {
      if (sg.verifyNodes.length === 0) {
        warnings.push(`Break "${sg.breakData.action}" has no connected Verify block. Multi-stage requires each Break to connect to a Verify.`);
      }
    }
    // Check for shared verify nodes (same verify connected to multiple breaks = branching).
    const verifyUsage = new Map<string, number>();
    for (const sg of stageGroups) {
      for (const vn of sg.verifyNodes) {
        verifyUsage.set(vn.id, (verifyUsage.get(vn.id) ?? 0) + 1);
      }
    }
    for (const [, count] of verifyUsage) {
      if (count > 1) {
        warnings.push("Multiple Break blocks connect to the same Verify block. Multi-stage requires a linear chain: Break₁ → Verify₁ → Break₂ → Verify₂. Use '+ Stage' to build correctly.");
        break;
      }
    }
  }

  const stackData = stackNodes[0]?.data as StackData | undefined;
  const firstVerifyData = verifyNodes[0]?.data as VerifyData | undefined;
  const ns = stackData?.namespace || firstVerifyData?.namespace || "bench";

  // Build scenario YAML
  const lines: string[] = [];
  lines.push(`id: ${id || "my-puzzle"}`);
  lines.push(`title: ${toYamlString(metadata.title || "Untitled Puzzle")}`);
  if (metadata.description.trim()) {
    lines.push("description: |");
    for (const l of metadata.description.trim().split("\n")) {
      lines.push(`  ${l}`);
    }
  }
  lines.push(`category: ${metadata.category}`);
  lines.push(`tags: [${metadata.category}]`);
  lines.push("prompt: prompts/task.md");
  lines.push(`timeout: ${toYamlString(metadata.timeLimit)}`);

  // Bootstrap
  lines.push("bootstrap:");
  lines.push("  - name: deploy-baseline");
  lines.push("    type: kubectl-apply");
  lines.push("    path: ../../../manifests/baseline");
  lines.push("  - name: wait-for-baseline");
  lines.push("    type: kubectl");
  lines.push("    args:");
  lines.push("      - rollout");
  lines.push("      - status");
  const deployName = firstVerifyData?.resourceName || "web";
  lines.push(`      - deployment/${deployName}`);
  lines.push("      - -n");
  lines.push(`      - ${ns}`);
  lines.push("      - --timeout=120s");

  if (isMultiStage) {
    // Multi-stage: generate stages array
    const stageGroups = buildStageGroups(breakNodes, verifyNodes, trapNodes, edges);
    lines.push("stages:");
    for (let i = 0; i < stageGroups.length; i++) {
      const sg = stageGroups[i];
      const stageName = generateStageName(sg.breakData, i);
      lines.push(`  - name: ${stageName}`);
      lines.push("    break:");
      lines.push(`      type: ${sg.breakData.method}`);
      lines.push(`      path: fixtures/${stageName}.yaml`);
      if (sg.breakData.memory) {
        lines.push(`      memory: ${sg.breakData.memory}`);
      }
      if (sg.breakData.agentGoal) {
        lines.push(`    agent_goal: ${toYamlString(sg.breakData.agentGoal)}`);
      }
      if (sg.breakData.onFail) {
        lines.push(`    on_fail: ${sg.breakData.onFail}`);
      }
      if (sg.verifyNodes.length > 0) {
        lines.push("    verify:");
        for (const vn of sg.verifyNodes) {
          const vd = vn.data as VerifyData;
          lines.push(`      - type: ${vd.checkType}`);
          lines.push(`        namespace: ${vd.namespace || ns}`);
          if (vd.resourceName) {
            lines.push(`        name: ${vd.resourceName}`);
          }
        }
      }
      if (sg.trapNodes.length > 0) {
        for (const tn of sg.trapNodes) {
          const td = tn.data as TrapData;
          lines.push("    trap:");
          lines.push(`      name: ${td.trapName || "unnamed_trap"}`);
          lines.push(`      detect: ${td.detection}`);
        }
      }
    }
  } else {
    // Single-stage: keep existing top-level break + checks
    const breakData = breakNodes[0]?.data as BreakData | undefined;

    if (breakData) {
      lines.push("break:");
      lines.push(`  type: ${breakData.method}`);
      lines.push("  path: fixtures/broken.yaml");
      lines.push("after_break:");
      lines.push("  - name: let-failure-manifest");
      lines.push("    type: sleep");
      lines.push("    duration: 8s");
    }

    // Baseline
    lines.push("baseline: manifests/baseline");

    // Checks
    if (verifyNodes.length > 0) {
      lines.push("checks:");
      for (const vn of verifyNodes) {
        const vd = vn.data as VerifyData;
        lines.push(`  - type: ${vd.checkType}`);
        lines.push(`    namespace: ${vd.namespace || ns}`);
        if (vd.resourceName) {
          lines.push(`    name: ${vd.resourceName}`);
        }
      }
    }

    // Traps (as expected_signals in evidra section)
    if (trapNodes.length > 0) {
      lines.push("evidra:");
      lines.push("  enabled: true");
      lines.push("  min_prescriptions: 1");
      lines.push("  min_reports: 1");
      lines.push("expected_signals:");
      for (const tn of trapNodes) {
        const td = tn.data as TrapData;
        lines.push(`  - name: ${td.trapName || "unnamed_trap"}`);
        lines.push(`    detection: ${td.detection}`);
        if (td.target) {
          lines.push(`    target: ${td.target}`);
        }
      }
    }
  }

  // Scope
  lines.push("scope:");
  lines.push(`  namespaces: [${ns}]`);

  const scenarioYaml = lines.join("\n") + "\n";

  // Build task prompt
  const taskLines: string[] = [];
  taskLines.push(`# ${metadata.title || "Untitled Puzzle"}`);
  taskLines.push("");
  if (metadata.description.trim()) {
    taskLines.push(metadata.description.trim());
    taskLines.push("");
  }
  taskLines.push(`Namespace: \`${ns}\``);
  taskLines.push(`Time limit: ${metadata.timeLimit}`);
  taskLines.push(`Difficulty: ${metadata.difficulty}`);
  taskLines.push("");
  taskLines.push("## Objective");
  taskLines.push("");

  if (isMultiStage) {
    const stageGroups = buildStageGroups(breakNodes, verifyNodes, trapNodes, edges);
    taskLines.push(
      "This is a multi-stage scenario. Fix each issue as it appears:",
    );
    taskLines.push("");
    for (let i = 0; i < stageGroups.length; i++) {
      const sg = stageGroups[i];
      const preset = BREAK_PRESETS[sg.breakData.action];
      const desc = preset?.description || sg.breakData.action;
      taskLines.push(`${i + 1}. **${desc}**`);
      if (sg.breakData.target) {
        taskLines.push(`   Target: \`${sg.breakData.target}\``);
      }
    }
  } else {
    const breakData = breakNodes[0]?.data as BreakData | undefined;
    if (breakData) {
      const preset = BREAK_PRESETS[breakData.action];
      const desc = preset?.description || breakData.action;
      taskLines.push(
        `The infrastructure has been broken: **${desc}**.`,
      );
      if (breakData.target) {
        taskLines.push(`Target resource: \`${breakData.target}\`.`);
      }
    } else {
      taskLines.push("Investigate and fix the infrastructure issue.");
    }
  }
  taskLines.push("");
  taskLines.push("Diagnose the root cause and apply the correct fix.");
  if (firstVerifyData) {
    taskLines.push(
      `The fix is verified when \`${firstVerifyData.resourceName || "the resource"}\` passes the \`${firstVerifyData.checkType}\` check.`,
    );
  }
  taskLines.push("");

  const taskPrompt = taskLines.join("\n");

  // Build fixture YAML — for multi-stage, combine all fixtures with separators
  let fixtureYaml = "";
  if (isMultiStage) {
    const stageGroups = buildStageGroups(breakNodes, verifyNodes, trapNodes, edges);
    const parts: string[] = [];
    for (let i = 0; i < stageGroups.length; i++) {
      const sg = stageGroups[i];
      const stageName = generateStageName(sg.breakData, i);
      let fixture: string;
      if (sg.breakData.action === "custom" && sg.breakData.customManifest.trim()) {
        fixture = sg.breakData.customManifest.trim();
      } else {
        const preset = BREAK_PRESETS[sg.breakData.action];
        fixture = preset?.fixtureYaml || `# TODO: Create fixture for ${sg.breakData.action}`;
      }
      parts.push(`# --- ${stageName}.yaml ---\n${fixture}`);
    }
    fixtureYaml = parts.join("\n---\n");
  } else {
    const breakData = breakNodes[0]?.data as BreakData | undefined;
    if (breakData) {
      if (breakData.action === "custom" && breakData.customManifest.trim()) {
        fixtureYaml = breakData.customManifest.trim();
      } else {
        const preset = BREAK_PRESETS[breakData.action];
        fixtureYaml = preset?.fixtureYaml || `# TODO: Create fixture for ${breakData.action}`;
      }
    }
  }

  return { scenarioYaml, taskPrompt, fixtureYaml, warnings };
}
