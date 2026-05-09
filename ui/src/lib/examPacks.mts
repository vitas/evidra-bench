export type ExamPackID =
  | "kubernetes-admin"
  | "kubernetes-security"
  | "gitops-release"
  | "terraform-cloud"
  | "mcp-readiness";
export type ExamPackFilter = "all" | ExamPackID;

export interface ExamPack {
  id: ExamPackID;
  title: string;
  shortTitle: string;
  summary: string;
  proof: string;
  filters: {
    categories?: string[];
    tracks?: string[];
    levels?: string[];
    includeChaos?: boolean;
  };
}

export interface ExamPackScenario {
  category: string;
  track?: string;
  level?: string;
  chaos?: boolean;
}

export const EXAM_PACKS: ExamPack[] = [
  {
    id: "kubernetes-admin",
    title: "Kubernetes Admin Exam",
    shortTitle: "K8s Admin",
    summary: "Workloads, troubleshooting, networking, and storage tasks in live Kubernetes environments.",
    proof: "Shows whether an agent can operate a cluster without guessing or over-mutating.",
    filters: {
      categories: ["kubernetes"],
      tracks: ["workloads", "troubleshooting", "networking", "storage"],
    },
  },
  {
    id: "kubernetes-security",
    title: "Kubernetes Security Exam",
    shortTitle: "K8s Security",
    summary: "RBAC, Pod Security, network policy, runtime, and credential exposure scenarios.",
    proof: "Shows whether an agent fixes security issues without weakening controls.",
    filters: {
      categories: ["kubernetes"],
      tracks: ["pod-security", "runtime-security"],
    },
  },
  {
    id: "gitops-release",
    title: "GitOps And Release Exam",
    shortTitle: "GitOps",
    summary: "Helm and Argo CD incidents covering drift, failed upgrades, rollback, and sync health.",
    proof: "Shows whether an agent can recover release systems while preserving declarative intent.",
    filters: {
      categories: ["helm", "argocd"],
      tracks: ["release-ops"],
    },
  },
  {
    id: "terraform-cloud",
    title: "Terraform And Cloud Ops Exam",
    shortTitle: "Terraform/Cloud",
    summary: "Terraform state, import, drift, AWS security controls, and cloud-operations recovery.",
    proof: "Shows whether an agent can reason about state and cloud controls before applying changes.",
    filters: {
      categories: ["terraform", "aws"],
      tracks: ["platform-eng"],
    },
  },
  {
    id: "mcp-readiness",
    title: "MCP Readiness Exam",
    shortTitle: "MCP Readiness",
    summary: "Non-trivial scenarios intended for baseline-vs-MCP comparison.",
    proof: "Shows whether a tool server improves diagnosis, safety, and cost under the same tasks.",
    filters: {
      levels: ["L2", "L3", "L4"],
      includeChaos: true,
    },
  },
];

function includesAny(allowed: string[] | undefined, value: string | undefined): boolean {
  return Boolean(value && allowed?.includes(value));
}

export function scenarioMatchesExamPack(
  scenario: ExamPackScenario,
  packID: ExamPackID,
): boolean {
  const pack = EXAM_PACKS.find((item) => item.id === packID);
  if (!pack) return false;

  if (pack.id === "mcp-readiness") {
    return includesAny(pack.filters.levels, scenario.level) || Boolean(scenario.chaos);
  }

  const categoryMatches = includesAny(pack.filters.categories, scenario.category);
  const trackMatches = includesAny(pack.filters.tracks, scenario.track);

  if (pack.id === "terraform-cloud") {
    return categoryMatches || trackMatches;
  }

  return categoryMatches && trackMatches;
}

export function countExamPackMatches(
  scenarios: ExamPackScenario[],
): Record<ExamPackID, number> {
  return Object.fromEntries(
    EXAM_PACKS.map((pack) => [
      pack.id,
      scenarios.filter((scenario) => scenarioMatchesExamPack(scenario, pack.id)).length,
    ]),
  ) as Record<ExamPackID, number>;
}

export function resolveExamPackFilter(value: string | null | undefined): ExamPackFilter {
  if (!value) return "all";
  return EXAM_PACKS.some((pack) => pack.id === value) ? (value as ExamPackID) : "all";
}
