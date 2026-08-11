CREATE TABLE creative_brand_brief_reviews (
    organization_id VARCHAR(96) NOT NULL,
    project_id VARCHAR(96) NOT NULL,
    intake_id VARCHAR(96) NOT NULL,
    input_identity_hash VARCHAR(71) NOT NULL,
    contract_version VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL,
    revision BIGINT NOT NULL,
    document_payload JSON NOT NULL,
    blockers JSON NOT NULL,
    warnings JSON NOT NULL,
    content_hash VARCHAR(71) NOT NULL,
    confirmed_by VARCHAR(96) NULL,
    confirmed_at DATETIME(6) NULL,
    created_by VARCHAR(96) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (organization_id, project_id, intake_id),
    UNIQUE KEY uk_creative_brand_brief_identity (
        organization_id, project_id, input_identity_hash
    )
);

ALTER TABLE creative_direction_batches
    ADD COLUMN brand_brief_revision BIGINT NULL AFTER input_identity_hash,
    ADD COLUMN brand_brief_content_hash VARCHAR(71) NULL AFTER brand_brief_revision;
