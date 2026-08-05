ALTER TABLE creative_intakes
    DROP CHECK chk_creative_intakes_source;

ALTER TABLE creative_intakes
    ADD CONSTRAINT chk_creative_intakes_source
        CHECK (source_type IN ('manual', 'strategy_package', 'task_strategy', 'uploaded_document', 'conversation', 'requirement_snapshot')),
    ADD COLUMN requirement_brief_id VARCHAR(96) NULL AFTER task_strategy_content_hash,
    ADD COLUMN requirement_brief_version BIGINT NULL AFTER requirement_brief_id,
    ADD COLUMN requirement_content_hash VARCHAR(71) NULL AFTER requirement_brief_version,
    ADD COLUMN business_code VARCHAR(96) NULL AFTER requirement_content_hash,
    ADD COLUMN business_version VARCHAR(64) NULL AFTER business_code,
    ADD COLUMN business_content_hash VARCHAR(71) NULL AFTER business_version;

CREATE INDEX idx_creative_intakes_requirement
    ON creative_intakes (
        organization_id, project_id, source_type,
        requirement_brief_id, requirement_brief_version, business_code
    );
