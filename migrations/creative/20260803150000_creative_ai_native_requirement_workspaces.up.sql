CREATE TABLE creative_ai_native_requirement_workspaces (
    organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    workspace_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    creative_intake_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    creative_task_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    status VARCHAR(32) NOT NULL,
    current_stage VARCHAR(32) NOT NULL,
    workspace_version BIGINT NOT NULL,
    active_operation_id VARCHAR(96) NULL,
    active_operation_version BIGINT NULL,
    current_revision BIGINT NOT NULL,
    confirmed_revision BIGINT NULL,
    created_by VARCHAR(96) NOT NULL,
    confirmed_by VARCHAR(96) NULL,
    confirmed_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (organization_id, project_id, workspace_id),
    UNIQUE KEY uq_ai_native_workspace_task (organization_id, creative_task_id),
    KEY idx_ai_native_requirement_project_updated (
        organization_id, project_id, updated_at
    ),
    CONSTRAINT fk_ai_native_workspace_intake
        FOREIGN KEY (organization_id, creative_intake_id)
        REFERENCES creative_intakes (organization_id, id),
    CONSTRAINT fk_ai_native_workspace_task
        FOREIGN KEY (organization_id, creative_task_id)
        REFERENCES creative_tasks (organization_id, id),
    CONSTRAINT chk_ai_native_workspace_version CHECK (workspace_version > 0),
	CONSTRAINT chk_ai_native_active_operation CHECK (
		(active_operation_id IS NULL AND active_operation_version IS NULL)
		OR (active_operation_id IS NOT NULL AND active_operation_version > 0)
	),
    CONSTRAINT chk_ai_native_workspace_stage CHECK (current_stage IN ('requirement')),
    CONSTRAINT chk_ai_native_workspace_status CHECK (status IN ('draft', 'confirmed'))
);

CREATE TABLE creative_ai_native_requirement_revisions (
    organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    workspace_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    revision BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL,
    content_payload JSON NOT NULL,
    content_hash CHAR(64) NOT NULL,
    based_on_revision BIGINT NULL,
    created_by VARCHAR(96) NOT NULL,
    confirmed_by VARCHAR(96) NULL,
    confirmed_at DATETIME(6) NULL,
    superseded_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (organization_id, project_id, workspace_id, revision),
    CONSTRAINT fk_ai_native_requirement_workspace
        FOREIGN KEY (organization_id, project_id, workspace_id)
        REFERENCES creative_ai_native_requirement_workspaces (
            organization_id, project_id, workspace_id
        ),
    CONSTRAINT chk_ai_native_requirement_revision CHECK (revision > 0),
    CONSTRAINT chk_ai_native_requirement_status CHECK (status IN ('draft', 'confirmed', 'superseded'))
);
