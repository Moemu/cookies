ALTER TABLE creative_ai_native_requirement_workspaces
    ADD COLUMN script_status VARCHAR(32) NULL AFTER confirmed_revision,
    ADD COLUMN current_script_revision BIGINT NULL AFTER script_status,
    ADD COLUMN confirmed_script_revision BIGINT NULL AFTER current_script_revision,
    ADD COLUMN script_error_code VARCHAR(96) NULL AFTER confirmed_script_revision,
    ADD COLUMN script_error_message VARCHAR(1000) NULL AFTER script_error_code,
    DROP CHECK chk_ai_native_workspace_stage,
    ADD CONSTRAINT chk_ai_native_workspace_stage CHECK (current_stage IN ('requirement', 'script')),
    ADD CONSTRAINT chk_ai_native_script_status CHECK (
        script_status IS NULL OR script_status IN ('generating', 'draft', 'confirmed', 'failed')
    );

CREATE TABLE creative_ai_native_script_revisions (
    organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    workspace_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    revision BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL,
    content_payload JSON NOT NULL,
    content_hash CHAR(64) NOT NULL,
    based_on_revision BIGINT NULL,
    based_on_requirement_revision BIGINT NOT NULL,
    based_on_requirement_hash CHAR(64) NOT NULL,
    channel_profile_id VARCHAR(96) NOT NULL,
    channel_profile_hash CHAR(64) NOT NULL,
    regeneration_note VARCHAR(2000) NULL,
    generation_metadata JSON NOT NULL,
    created_by VARCHAR(96) NOT NULL,
    confirmed_by VARCHAR(96) NULL,
    confirmed_at DATETIME(6) NULL,
    superseded_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (organization_id, project_id, workspace_id, revision),
    CONSTRAINT fk_ai_native_script_workspace
        FOREIGN KEY (organization_id, project_id, workspace_id)
        REFERENCES creative_ai_native_requirement_workspaces (
            organization_id, project_id, workspace_id
        ),
    CONSTRAINT chk_ai_native_script_revision CHECK (revision > 0),
    CONSTRAINT chk_ai_native_script_revision_status CHECK (status IN ('draft', 'confirmed', 'superseded'))
);
