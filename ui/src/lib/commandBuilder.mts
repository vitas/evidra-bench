export type EvidenceModeId = "baseline" | "evidra-mcp";

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
    id: "evidra-mcp",
    label: "evidra-mcp",
    description: "Generic MCP server",
  },
];

function evidenceFlags(evidenceMode: EvidenceModeId): string[] {
  if (evidenceMode === "evidra-mcp") {
    return ['--mcp-server "evidra-mcp --signing-mode optional"'];
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
    "  --evidra-url $EVIDRA_URL \\",
    "  --evidra-api-key $EVIDRA_API_KEY",
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
    "--evidra-url $EVIDRA_URL",
    "--evidra-api-key $EVIDRA_API_KEY",
  ];
  return lines.join(" \\\n  ");
}
