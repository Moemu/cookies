CREATE TABLE IF NOT EXISTS delivery_plans (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  platform VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  scenario VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  current_version INT NOT NULL,
  created_by_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_by_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (organization_id, id),
  INDEX idx_delivery_plans_project_updated (organization_id, project_id, updated_at, id),
  CONSTRAINT chk_delivery_plans_current_version CHECK (current_version >= 1),
  CONSTRAINT chk_delivery_plans_source CHECK (source = 'mock')
);

CREATE TABLE IF NOT EXISTS delivery_plan_versions (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  plan_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  version_number INT NOT NULL,
  config_json JSON NOT NULL,
  source VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  scenario VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_by_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_by_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (organization_id, plan_id, version_number),
  INDEX idx_delivery_plan_versions_project (organization_id, project_id, plan_id, version_number),
  CONSTRAINT fk_delivery_plan_versions_plan
    FOREIGN KEY (organization_id, plan_id) REFERENCES delivery_plans(organization_id, id),
  CONSTRAINT chk_delivery_plan_versions_number CHECK (version_number >= 1),
  CONSTRAINT chk_delivery_plan_versions_config_json CHECK (JSON_VALID(config_json)),
  CONSTRAINT chk_delivery_plan_versions_source CHECK (source = 'mock')
);
