ALTER TABLE bench_runs ALTER COLUMN evidence_mode SET DEFAULT 'none';

UPDATE bench_runs
SET evidence_mode = 'mcp'
WHERE evidence_mode NOT IN ('none', 'mcp');
