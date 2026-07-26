ALTER TABLE job_segments ADD COLUMN prompt_duration_ms INTEGER NOT NULL DEFAULT 0;
ALTER TABLE job_segments ADD COLUMN output_duration_ms INTEGER NOT NULL DEFAULT 0;
