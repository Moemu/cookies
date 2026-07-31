CREATE TABLE creative_image_generation_attempts (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  task_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  draft_revision BIGINT NOT NULL,
  image_plan_order INT NOT NULL,
  attempt_no INT NOT NULL,
  prompt_package_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  generation_spec_payload JSON NOT NULL,
  generation_spec_hash VARCHAR(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  provider_job_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  render_job_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  status VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  base_asset_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  base_asset_version BIGINT NULL,
  final_asset_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  final_asset_version BIGINT NULL,
  reused_from_attempt_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  stale_reason VARCHAR(255) NOT NULL DEFAULT '',
  error_code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  error_message VARCHAR(1000) NOT NULL DEFAULT '',
  idempotency_key VARCHAR(192) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  request_hash VARCHAR(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_by_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_by VARCHAR(192) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (organization_id, id),
  UNIQUE KEY uq_creative_image_attempt_number
    (organization_id, task_id, draft_revision, image_plan_order, attempt_no),
  UNIQUE KEY uq_creative_image_attempt_idempotency
    (organization_id, project_id, idempotency_key),
  UNIQUE KEY uq_creative_image_attempt_provider
    (organization_id, provider_job_id),
  KEY idx_creative_image_attempt_task
    (organization_id, project_id, task_id, draft_revision, image_plan_order, created_at),
  KEY idx_creative_image_attempt_recovery (status, updated_at),
  CONSTRAINT fk_creative_image_attempt_task
    FOREIGN KEY (organization_id, task_id) REFERENCES creative_tasks(organization_id, id),
  CONSTRAINT fk_creative_image_attempt_prompt
    FOREIGN KEY (organization_id, prompt_package_id) REFERENCES creative_image_prompt_packages(organization_id, id),
  CONSTRAINT chk_creative_image_attempt_revision CHECK (draft_revision > 0),
  CONSTRAINT chk_creative_image_attempt_order CHECK (image_plan_order BETWEEN 1 AND 3),
  CONSTRAINT chk_creative_image_attempt_number CHECK (attempt_no > 0),
  CONSTRAINT chk_creative_image_attempt_status CHECK (
    status IN ('queued', 'running', 'base_asset_ready', 'rendering', 'succeeded', 'failed', 'cancelled', 'stale')
  ),
  CONSTRAINT chk_creative_image_attempt_base_ref CHECK (
    (base_asset_id IS NULL AND base_asset_version IS NULL)
    OR (base_asset_id IS NOT NULL AND base_asset_version > 0)
  ),
  CONSTRAINT chk_creative_image_attempt_final_ref CHECK (
    (final_asset_id IS NULL AND final_asset_version IS NULL)
    OR (final_asset_id IS NOT NULL AND final_asset_version > 0)
  )
);

CREATE TABLE creative_image_slot_selections (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  task_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  draft_revision BIGINT NOT NULL,
  image_plan_order INT NOT NULL,
  adopted_attempt_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  version BIGINT NOT NULL,
  adopted_by VARCHAR(192) NOT NULL,
  adopted_at DATETIME(6) NOT NULL,
  PRIMARY KEY (organization_id, task_id, draft_revision, image_plan_order),
  CONSTRAINT fk_creative_image_selection_task
    FOREIGN KEY (organization_id, task_id) REFERENCES creative_tasks(organization_id, id),
  CONSTRAINT fk_creative_image_selection_attempt
    FOREIGN KEY (organization_id, adopted_attempt_id) REFERENCES creative_image_generation_attempts(organization_id, id),
  CONSTRAINT chk_creative_image_selection_revision CHECK (draft_revision > 0),
  CONSTRAINT chk_creative_image_selection_order CHECK (image_plan_order BETWEEN 1 AND 3),
  CONSTRAINT chk_creative_image_selection_version CHECK (version > 0)
);
