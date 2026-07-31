CREATE TABLE creative_commerce_preroll_workspaces (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  fixture_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  fixture_version BIGINT NOT NULL,
  template_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  intake_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  task_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (organization_id, project_id, fixture_id, fixture_version),
  UNIQUE KEY uq_creative_commerce_workspace_task (organization_id, task_id),
  CONSTRAINT fk_creative_commerce_workspace_intake
    FOREIGN KEY (organization_id, intake_id) REFERENCES creative_intakes(organization_id, id),
  CONSTRAINT fk_creative_commerce_workspace_task
    FOREIGN KEY (organization_id, task_id) REFERENCES creative_tasks(organization_id, id),
  CONSTRAINT chk_creative_commerce_workspace_fixture_version CHECK (fixture_version >= 1)
);

CREATE TABLE creative_commerce_preroll_generation_attempts (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  task_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  draft_revision BIGINT NOT NULL,
  template_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  template_version BIGINT NOT NULL,
  prompt_hash VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  generation_spec_hash VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  provider_job_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  retry_of_attempt_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  output_asset_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  output_asset_version BIGINT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uq_creative_commerce_attempt_provider_job (organization_id, provider_job_id),
  KEY idx_creative_commerce_attempt_task (organization_id, project_id, task_id, created_at),
  CONSTRAINT fk_creative_commerce_attempt_task
    FOREIGN KEY (organization_id, task_id) REFERENCES creative_tasks(organization_id, id),
  CONSTRAINT fk_creative_commerce_attempt_retry
    FOREIGN KEY (retry_of_attempt_id) REFERENCES creative_commerce_preroll_generation_attempts(id),
  CONSTRAINT chk_creative_commerce_attempt_draft_revision CHECK (draft_revision >= 1),
  CONSTRAINT chk_creative_commerce_attempt_template_version CHECK (template_version >= 1),
  CONSTRAINT chk_creative_commerce_attempt_output CHECK (
    (output_asset_id IS NULL AND output_asset_version IS NULL)
    OR (output_asset_id IS NOT NULL AND output_asset_version >= 1)
  )
);
