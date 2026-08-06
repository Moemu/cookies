ALTER TABLE creative_ai_native_requirement_workspaces
    ADD COLUMN display_name VARCHAR(120) NOT NULL DEFAULT '' AFTER workspace_id;

CREATE INDEX idx_ai_native_workspace_catalog
    ON creative_ai_native_requirement_workspaces (
        organization_id,
        project_id,
        updated_at,
        workspace_id
    );
