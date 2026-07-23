CREATE TABLE creative_intakes (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  principal_kind VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  principal_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_type VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  request_payload JSON NOT NULL,
  missing_fields JSON NOT NULL,
  warnings JSON NOT NULL,
  confirmed_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  idempotency_key VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  request_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_creative_intakes_org_id (organization_id, id),
  UNIQUE KEY uq_creative_intakes_idempotency (organization_id, project_id, principal_kind, principal_id, idempotency_key),
  KEY idx_creative_intakes_scope (organization_id, project_id, created_at),
  CONSTRAINT fk_creative_intakes_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT chk_creative_intakes_source CHECK (source_type IN ('manual', 'strategy_package', 'uploaded_document', 'conversation')),
  CONSTRAINT chk_creative_intakes_status CHECK (status IN ('draft', 'needs_clarification', 'ready', 'superseded')),
  CONSTRAINT chk_creative_intakes_principal_kind CHECK (principal_kind IN ('user', 'service')),
  CONSTRAINT chk_creative_intakes_version CHECK (version > 0)
);

CREATE TABLE creative_tasks (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  intake_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  creative_format VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  channel VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  direction_payload JSON NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_creative_tasks_org_id (organization_id, id),
  KEY idx_creative_tasks_scope (organization_id, project_id, created_at),
  KEY idx_creative_tasks_intake (organization_id, project_id, intake_id),
  CONSTRAINT fk_creative_tasks_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT fk_creative_tasks_intake FOREIGN KEY (organization_id, intake_id) REFERENCES creative_intakes(organization_id, id),
  CONSTRAINT chk_creative_tasks_format CHECK (creative_format IN ('image_text')),
  CONSTRAINT chk_creative_tasks_channel CHECK (channel IN ('xiaohongshu')),
  CONSTRAINT chk_creative_tasks_status CHECK (status IN ('draft', 'in_progress', 'ready_for_review')),
  CONSTRAINT chk_creative_tasks_version CHECK (version > 0)
);

CREATE TABLE creative_image_text_drafts (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  task_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  version BIGINT NOT NULL,
  status VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  content_payload JSON NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (organization_id, task_id, version),
  CONSTRAINT fk_creative_drafts_task FOREIGN KEY (organization_id, task_id) REFERENCES creative_tasks(organization_id, id),
  CONSTRAINT chk_creative_drafts_version CHECK (version > 0),
  CONSTRAINT chk_creative_drafts_status CHECK (status IN ('draft', 'ready_for_review', 'approved', 'superseded'))
);

CREATE TABLE creative_production_jobs (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  task_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  job_kind VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  provider_job_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (organization_id, task_id, job_kind),
  UNIQUE KEY uq_creative_production_provider_job (organization_id, provider_job_id),
  CONSTRAINT fk_creative_production_task FOREIGN KEY (organization_id, task_id) REFERENCES creative_tasks(organization_id, id),
  CONSTRAINT chk_creative_production_kind CHECK (job_kind IN ('cover_image'))
);
