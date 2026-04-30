-- Add tool_server column to bench_runs for tracking which MCP server was used.
-- Empty string = baseline (direct exec, no MCP server).
-- Examples: "evidra-mcp", "kubectl-mcp-server", "kagent"
ALTER TABLE bench_runs ADD COLUMN IF NOT EXISTS tool_server TEXT NOT NULL DEFAULT '';
