export type ToolBackendId = "baseline" | "mcp";

export const TOOL_BACKENDS: Array<{
  id: ToolBackendId;
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

function toolBackendFlags(toolBackend: ToolBackendId): string[] {
  if (toolBackend === "mcp") {
    return [
      '--mcp-server "$MCP_SERVER"',
      '--tool-server-id "$TOOL_SERVER_ID"',
      '--tool-server-version "$TOOL_SERVER_VERSION"',
    ];
  }
  return [];
}

export function buildBenchCommand(input: {
  scenarios: string[];
  model: string;
  toolBackend: ToolBackendId;
}): string {
  const lines = [
    "bench-cli bench \\",
    ...input.scenarios.map((scenario) => `  --scenario ${scenario} \\`),
    `  --model ${input.model} \\`,
    "  --provider bifrost \\",
    ...toolBackendFlags(input.toolBackend).map((flag) => `  ${flag} \\`),
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
  toolBackend: ToolBackendId;
}): string {
  const lines = [
    "bench-cli run",
    `--scenario ${input.scenario}`,
    `--model ${input.model}`,
    "--provider bifrost",
    ...toolBackendFlags(input.toolBackend),
    "--reuse-cluster",
    "--timeout 5m",
    "--bench-url $BENCH_API_URL",
    "--bench-api-key $BENCH_API_KEY",
  ];
  return lines.join(" \\\n  ");
}
