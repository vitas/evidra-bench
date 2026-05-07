export type EvidenceModeId = "baseline" | "mcp";

export const EVIDENCE_MODES: Array<{
  id: EvidenceModeId;
  label: string;
  description: string;
}> = [
  {
    id: "baseline",
    label: "Baseline",
    description: "Direct execution",
  },
  {
    id: "mcp",
    label: "MCP server",
    description: "Tool server",
  },
];

function evidenceFlags(evidenceMode: EvidenceModeId): string[] {
  if (evidenceMode === "mcp") {
    return ['--mcp-server "$MCP_SERVER"'];
  }
  return [];
}

export function buildBenchCommand(input: {
  scenarios: string[];
  model: string;
  evidenceMode: EvidenceModeId;
}): string {
  const lines = [
    "bench-cli bench \\",
    ...input.scenarios.map((scenario) => `  --scenario ${scenario} \\`),
    `  --model ${input.model} \\`,
    "  --provider bifrost \\",
    ...evidenceFlags(input.evidenceMode).map((flag) => `  ${flag} \\`),
    "  --reuse-cluster \\",
    "  --timeout 5m \\",
    "  --bench-url $BENCH_API_URL \\",
    "  --bench-api-key $BENCH_API_KEY",
  ];
  return lines.join("\n");
}

export function buildRunCommand(input: {
  scenario: string;
  model: string;
  evidenceMode: EvidenceModeId;
}): string {
  const lines = [
    "bench-cli run",
    `--scenario ${input.scenario}`,
    `--model ${input.model}`,
    "--provider bifrost",
    ...evidenceFlags(input.evidenceMode),
    "--reuse-cluster",
    "--timeout 5m",
    "--bench-url $BENCH_API_URL",
    "--bench-api-key $BENCH_API_KEY",
  ];
  return lines.join(" \\\n  ");
}
