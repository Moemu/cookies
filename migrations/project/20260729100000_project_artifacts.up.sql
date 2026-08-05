CREATE TABLE IF NOT EXISTS platform_project_artifacts (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  kind VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'draft',
  content MEDIUMTEXT NOT NULL,
  source_job_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (organization_id, project_id, id),
  KEY idx_platform_project_artifacts_project_kind (organization_id, project_id, kind, updated_at),
  CONSTRAINT fk_platform_project_artifacts_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT chk_platform_project_artifacts_kind CHECK (kind IN ('brief', 'image', 'video', 'document')),
  CONSTRAINT chk_platform_project_artifacts_status CHECK (status IN ('draft', 'ready', 'archived')),
  CONSTRAINT chk_platform_project_artifacts_version CHECK (version > 0)
);
