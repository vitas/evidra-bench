-- Add track and level columns to bench_scenarios for CKA/CKS classification
ALTER TABLE bench_scenarios ADD COLUMN IF NOT EXISTS track TEXT NOT NULL DEFAULT '';
ALTER TABLE bench_scenarios ADD COLUMN IF NOT EXISTS level TEXT NOT NULL DEFAULT '';
