DROP TABLE IF EXISTS creative_ai_native_storyboard_revisions;

ALTER TABLE creative_ai_native_requirement_workspaces
    DROP CHECK chk_ai_native_storyboard_status,
    DROP CHECK chk_ai_native_workspace_stage,
    DROP COLUMN storyboard_error_message,
    DROP COLUMN storyboard_error_code,
    DROP COLUMN storyboard_plan_payload,
    DROP COLUMN confirmed_storyboard_revision,
    DROP COLUMN current_storyboard_revision,
    DROP COLUMN storyboard_status,
    ADD CONSTRAINT chk_ai_native_workspace_stage CHECK (current_stage IN ('requirement', 'script'));
