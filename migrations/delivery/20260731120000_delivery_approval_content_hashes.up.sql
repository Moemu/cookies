-- A03 binds every immutable PlanVersion and mock execution approval to exact
-- canonical content. Existing rows are populated immediately after SQL
-- migration by delivery.BackfillPlanCanonicalHashes using the shared Go RFC
-- 8785 JCS + SHA-256 implementation.
ALTER TABLE delivery_plan_versions
  ADD COLUMN canonical_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER config_json;

CREATE TABLE delivery_approvals (
  approval_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  plan_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  plan_version BIGINT NOT NULL,
  change_set_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  change_set_version BIGINT NOT NULL,
  plan_canonical_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  action_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  action VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  scope VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  budget_limit_minor BIGINT NOT NULL,
  currency CHAR(3) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  approved_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  approved_at DATETIME(6) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  source VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  scenario VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  UNIQUE KEY uq_delivery_approvals_change_set (organization_id, change_set_id),
  KEY idx_delivery_approvals_project (organization_id, project_id, approved_at),
  KEY idx_delivery_approvals_plan (organization_id, project_id, plan_id, plan_version),
  CONSTRAINT fk_delivery_approvals_project
    FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT fk_delivery_approvals_plan
    FOREIGN KEY (organization_id, plan_id) REFERENCES delivery_plans(organization_id, id),
  CONSTRAINT fk_delivery_approvals_change_set
    FOREIGN KEY (organization_id, change_set_id) REFERENCES delivery_change_sets(organization_id, id),
  CONSTRAINT chk_delivery_approvals_plan_version CHECK (plan_version > 0),
  CONSTRAINT chk_delivery_approvals_change_set_version CHECK (change_set_version > 0),
  CONSTRAINT chk_delivery_approvals_action CHECK (action = 'execute'),
  CONSTRAINT chk_delivery_approvals_scope CHECK (scope = 'execute_mock'),
  CONSTRAINT chk_delivery_approvals_budget CHECK (budget_limit_minor >= 0),
  CONSTRAINT chk_delivery_approvals_currency CHECK (currency = 'CNY'),
  CONSTRAINT chk_delivery_approvals_expiry CHECK (expires_at > approved_at),
  CONSTRAINT chk_delivery_approvals_source CHECK (source = 'mock')
);
