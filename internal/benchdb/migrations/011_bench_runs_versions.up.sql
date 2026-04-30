-- Add version tracking columns to bench_runs.
-- tool_server_version: version of the MCP server binary used (e.g. "0.5.0").
-- scenario_version: version/hash of the scenario definition at run time.
ALTER TABLE bench_runs ADD COLUMN IF NOT EXISTS tool_server_version TEXT NOT NULL DEFAULT '';
ALTER TABLE bench_runs ADD COLUMN IF NOT EXISTS scenario_version TEXT NOT NULL DEFAULT '';
