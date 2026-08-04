ALTER TABLE creative_ai_native_requirement_workspaces
    DROP CHECK chk_ai_native_production_status,
    ADD CONSTRAINT chk_ai_native_production_status CHECK (
        production_status IS NULL OR production_status IN ('running', 'assets_ready', 'failed', 'cancelled')
    );
