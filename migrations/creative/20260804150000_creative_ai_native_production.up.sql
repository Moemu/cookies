ALTER TABLE creative_ai_native_requirement_workspaces
    ADD COLUMN production_status VARCHAR(32) NULL AFTER storyboard_error_message,
    ADD COLUMN current_production_revision BIGINT NULL AFTER production_status,
    ADD COLUMN production_plan_payload JSON NULL AFTER current_production_revision,
    ADD COLUMN production_error_code VARCHAR(96) NULL AFTER production_plan_payload,
    ADD COLUMN production_error_message VARCHAR(1000) NULL AFTER production_error_code,
    DROP CHECK chk_ai_native_workspace_stage,
    ADD CONSTRAINT chk_ai_native_workspace_stage CHECK (current_stage IN ('requirement', 'script', 'storyboard', 'production')),
    ADD CONSTRAINT chk_ai_native_production_status CHECK (
        production_status IS NULL OR production_status IN ('running', 'assets_ready', 'rendering', 'completed', 'render_failed', 'failed', 'cancelled')
    );
