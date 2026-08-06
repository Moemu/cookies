CREATE TABLE IF NOT EXISTS platform_media_understanding_artifacts (
    id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin PRIMARY KEY,
    organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    asset_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    asset_version BIGINT NOT NULL,
    asset_kind VARCHAR(16) NOT NULL,
    asset_sha256 CHAR(64) NOT NULL,
    profile VARCHAR(64) NOT NULL,
    profile_version VARCHAR(32) NOT NULL,
    input_identity_hash CHAR(64) NOT NULL,
    model_route_revision VARCHAR(96) NULL,
    status VARCHAR(16) NOT NULL,
    job_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
    artifact_json JSON NOT NULL,
    content_hash CHAR(64) NOT NULL,
    error_code VARCHAR(96) NULL,
    error_message VARCHAR(1024) NULL,
    created_by_kind VARCHAR(16) NOT NULL,
    created_by_id VARCHAR(96) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    UNIQUE KEY uq_platform_media_understanding_identity
        (organization_id, project_id, input_identity_hash),
    KEY idx_platform_media_understanding_asset
        (organization_id, project_id, asset_id, asset_version, updated_at),
    KEY idx_platform_media_understanding_status
        (organization_id, status, updated_at),
    CONSTRAINT fk_platform_media_understanding_project
        FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
    CONSTRAINT fk_platform_media_understanding_asset
        FOREIGN KEY (organization_id, asset_id, asset_version) REFERENCES asset_versions(organization_id, asset_id, version),
    CONSTRAINT chk_platform_media_understanding_kind
        CHECK (asset_kind IN ('image', 'video')),
    CONSTRAINT chk_platform_media_understanding_status
        CHECK (status IN ('running', 'ready', 'partial', 'failed'))
);
