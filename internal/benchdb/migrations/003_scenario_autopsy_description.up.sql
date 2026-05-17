ALTER TABLE bench_scenarios
    ADD COLUMN IF NOT EXISTS autopsy_description TEXT NOT NULL DEFAULT '';
