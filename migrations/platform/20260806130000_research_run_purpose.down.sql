ALTER TABLE platform_research_runs
    DROP INDEX idx_research_runs_source,
    DROP COLUMN source_id,
    DROP COLUMN source_type,
    DROP COLUMN purpose;
