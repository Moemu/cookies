DROP INDEX idx_ai_native_workspace_catalog
    ON creative_ai_native_requirement_workspaces;

ALTER TABLE creative_ai_native_requirement_workspaces
    DROP COLUMN display_name;
