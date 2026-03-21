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

export function generateScenario(
  nodes: Node[],
  _edges: Edge[],
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

  const breakData = breakNodes[0]?.data as BreakData | undefined;
  const verifyData = verifyNodes[0]?.data as VerifyData | undefined;
  const stackData = stackNodes[0]?.data as StackData | undefined;
  const ns = stackData?.namespace || verifyData?.namespace || "bench";

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
  const deployName = verifyData?.resourceName || "web";
  lines.push(`      - deployment/${deployName}`);
  lines.push("      - -n");
  lines.push(`      - ${ns}`);
  lines.push("      - --timeout=120s");

  // Break
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
  taskLines.push("");
  taskLines.push("Diagnose the root cause and apply the correct fix.");
  if (verifyData) {
    taskLines.push(
      `The fix is verified when \`${verifyData.resourceName || "the resource"}\` passes the \`${verifyData.checkType}\` check.`,
    );
  }
  taskLines.push("");

  const taskPrompt = taskLines.join("\n");

  // Build fixture YAML
  let fixtureYaml = "";
  if (breakData) {
    if (breakData.action === "custom" && breakData.customManifest.trim()) {
      fixtureYaml = breakData.customManifest.trim();
    } else {
      const preset = BREAK_PRESETS[breakData.action];
      fixtureYaml = preset?.fixtureYaml || `# TODO: Create fixture for ${breakData.action}`;
    }
  }

  return { scenarioYaml, taskPrompt, fixtureYaml, warnings };
}
