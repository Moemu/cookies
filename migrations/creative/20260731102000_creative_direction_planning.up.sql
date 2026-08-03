CREATE TABLE creative_direction_batches (
    organization_id VARCHAR(96) NOT NULL,
    project_id VARCHAR(96) NOT NULL,
    batch_id VARCHAR(96) NOT NULL,
    intake_id VARCHAR(96) NOT NULL,
    input_identity_hash VARCHAR(71) NOT NULL,
    status VARCHAR(32) NOT NULL,
    model VARCHAR(191) NOT NULL,
    prompt_version VARCHAR(64) NOT NULL,
    failure_code VARCHAR(191) NULL,
    created_by VARCHAR(96) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (organization_id, project_id, batch_id),
    KEY idx_creative_direction_batch_intake (
        organization_id, project_id, intake_id, created_at
    )
);

CREATE TABLE creative_directions (
    organization_id VARCHAR(96) NOT NULL,
    project_id VARCHAR(96) NOT NULL,
    direction_id VARCHAR(96) NOT NULL,
    batch_id VARCHAR(96) NOT NULL,
    intake_id VARCHAR(96) NOT NULL,
    input_identity_hash VARCHAR(71) NOT NULL,
    route_id VARCHAR(96) NOT NULL,
    status VARCHAR(32) NOT NULL,
    version BIGINT NOT NULL,
    snapshot JSON NOT NULL,
    content_hash VARCHAR(71) NOT NULL,
    confirmed_by VARCHAR(96) NULL,
    confirmed_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (organization_id, project_id, direction_id),
    KEY idx_creative_direction_batch (
        organization_id, project_id, batch_id
    ),
    KEY idx_creative_direction_intake_status (
        organization_id, project_id, intake_id, status
    ),
    CONSTRAINT fk_creative_direction_batch
        FOREIGN KEY (organization_id, project_id, batch_id)
        REFERENCES creative_direction_batches (organization_id, project_id, batch_id)
);
