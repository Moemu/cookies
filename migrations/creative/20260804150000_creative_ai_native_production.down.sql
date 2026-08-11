ALTER TABLE creative_ai_native_requirement_workspaces
    DROP CHECK chk_ai_native_production_status,
    DROP CHECK chk_ai_native_workspace_stage,
    DROP COLUMN production_error_message,
    DROP COLUMN production_error_code,
    DROP COLUMN production_plan_payload,
    DROP COLUMN current_production_revision,
    DROP COLUMN production_status,
    ADD CONSTRAINT chk_ai_native_workspace_stage CHECK (current_stage IN ('requirement', 'script', 'storyboard'));
