ALTER TABLE platform_research_runs
    ADD COLUMN purpose VARCHAR(32) NOT NULL DEFAULT 'deep_research' AFTER category,
    ADD COLUMN source_type VARCHAR(64) NULL AFTER purpose,
    ADD COLUMN source_id VARCHAR(96) NULL AFTER source_type,
    ADD INDEX idx_research_runs_source (organization_id, project_id, source_type, source_id, created_at);
