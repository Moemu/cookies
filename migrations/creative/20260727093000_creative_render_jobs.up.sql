CREATE TABLE creative_render_jobs (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  task_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  pre_roll_asset_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  pre_roll_asset_version BIGINT NOT NULL,
  main_asset_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  main_asset_version BIGINT NOT NULL,
  output_asset_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  output_asset_version BIGINT NULL,
  error_code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  error_message VARCHAR(1000) NULL,
  created_by_kind VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_by_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  idempotency_key VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  request_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_creative_render_idempotency
    (organization_id, project_id, created_by_kind, created_by_id, idempotency_key),
  KEY idx_creative_render_task (organization_id, project_id, task_id, created_at),
  CONSTRAINT fk_creative_render_task
    FOREIGN KEY (organization_id, task_id) REFERENCES creative_tasks(organization_id, id),
  CONSTRAINT chk_creative_render_status CHECK (status IN ('queued', 'running', 'succeeded', 'failed')),
  CONSTRAINT chk_creative_render_asset_versions CHECK (
    pre_roll_asset_version > 0 AND main_asset_version > 0
    AND (output_asset_version IS NULL OR output_asset_version > 0)
  )
);
