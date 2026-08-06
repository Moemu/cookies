CREATE TABLE creative_edit_render_jobs (
    organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    edit_render_job_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    edit_task_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    timeline_version BIGINT NOT NULL,
    timeline_payload JSON NOT NULL,
    timeline_hash CHAR(71) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    kind VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    status VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    progress_percent TINYINT UNSIGNED NOT NULL DEFAULT 0,
    output_asset_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
    output_asset_version BIGINT NULL,
    error_code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
    error_message VARCHAR(1000) NULL,
    retry_of VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
    created_by_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    created_by_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (organization_id, project_id, edit_render_job_id),
    KEY idx_creative_edit_render_task (organization_id, project_id, edit_task_id, created_at),
    CONSTRAINT fk_creative_edit_render_task FOREIGN KEY (organization_id, project_id, edit_task_id)
      REFERENCES creative_edit_tasks (organization_id, project_id, edit_task_id),
    CONSTRAINT chk_creative_edit_render_kind CHECK (kind IN ('preview', 'export')),
    CONSTRAINT chk_creative_edit_render_status CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    CONSTRAINT chk_creative_edit_render_progress CHECK (progress_percent <= 100),
    CONSTRAINT chk_creative_edit_render_output CHECK (
      (output_asset_id IS NULL AND output_asset_version IS NULL) OR
      (output_asset_id IS NOT NULL AND output_asset_version IS NOT NULL)
    )
);
