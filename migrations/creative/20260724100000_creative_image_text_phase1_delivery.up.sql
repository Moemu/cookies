ALTER TABLE creative_production_jobs
  DROP CHECK chk_creative_production_kind;

ALTER TABLE creative_production_jobs
  ADD CONSTRAINT chk_creative_production_kind
  CHECK (job_kind = 'cover_image' OR job_kind REGEXP '^image_plan_([1-9]|1[0-2])$');

ALTER TABLE creative_versions
  ADD COLUMN check_payload JSON NULL AFTER created_at,
  ADD COLUMN approval_payload JSON NULL AFTER check_payload;

CREATE TABLE creative_packages (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  creative_version_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  content_hash CHAR(71) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  snapshot_payload JSON NOT NULL,
  created_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_creative_packages_version (organization_id, creative_version_id),
  KEY idx_creative_packages_scope (organization_id, project_id, created_at),
  CONSTRAINT fk_creative_packages_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT fk_creative_packages_version FOREIGN KEY (organization_id, creative_version_id) REFERENCES creative_versions(organization_id, id)
);
