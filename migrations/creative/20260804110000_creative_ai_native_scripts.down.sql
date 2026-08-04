DROP TABLE IF EXISTS creative_ai_native_script_revisions;

ALTER TABLE creative_ai_native_requirement_workspaces
    DROP CHECK chk_ai_native_script_status,
    DROP CHECK chk_ai_native_workspace_stage,
    DROP COLUMN script_error_message,
    DROP COLUMN script_error_code,
    DROP COLUMN confirmed_script_revision,
    DROP COLUMN current_script_revision,
    DROP COLUMN script_status,
    ADD CONSTRAINT chk_ai_native_workspace_stage CHECK (current_stage IN ('requirement'));
