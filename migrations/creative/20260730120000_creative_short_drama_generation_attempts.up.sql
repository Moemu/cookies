CREATE TABLE creative_short_drama_generation_attempts (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  task_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  draft_revision BIGINT NOT NULL,
  candidate_batch_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  candidate_id VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  prompt_package_hash VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  generation_spec_hash VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  provider_job_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  output_asset_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  output_asset_version BIGINT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uq_creative_short_drama_attempt_provider_job (organization_id, provider_job_id),
  KEY idx_creative_short_drama_attempt_task (organization_id, project_id, task_id, created_at),
  CONSTRAINT fk_creative_short_drama_attempt_task
    FOREIGN KEY (organization_id, task_id) REFERENCES creative_tasks(organization_id, id),
  CONSTRAINT chk_creative_short_drama_attempt_draft_revision CHECK (draft_revision >= 1),
  CONSTRAINT chk_creative_short_drama_attempt_output CHECK (
    (output_asset_id IS NULL AND output_asset_version IS NULL)
    OR (output_asset_id IS NOT NULL AND output_asset_version >= 1)
  )
);
