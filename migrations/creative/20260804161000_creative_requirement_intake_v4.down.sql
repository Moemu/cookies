DROP INDEX idx_creative_intakes_requirement ON creative_intakes;

ALTER TABLE creative_intakes
    DROP CHECK chk_creative_intakes_source;

ALTER TABLE creative_intakes
    ADD CONSTRAINT chk_creative_intakes_source
        CHECK (source_type IN ('manual', 'strategy_package', 'task_strategy', 'uploaded_document', 'conversation')),
    DROP COLUMN business_content_hash,
    DROP COLUMN business_version,
    DROP COLUMN business_code,
    DROP COLUMN requirement_content_hash,
    DROP COLUMN requirement_brief_version,
    DROP COLUMN requirement_brief_id;
