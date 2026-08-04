-- Three-tier delivery configuration is forward-only and additive. Existing plans, ChangeSets and approvals
-- continue to resolve through their immutable PlanVersion snapshots.
ALTER TABLE delivery_change_sets
  ADD COLUMN target_snapshot JSON NULL AFTER preflight_notes,
  ADD COLUMN target_snapshot_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER target_snapshot,
  ADD COLUMN recommendation_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER target_snapshot_hash;
ALTER TABLE delivery_approvals
  ADD COLUMN target_snapshot_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER plan_canonical_hash;

CREATE TABLE IF NOT EXISTS delivery_recommendations (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  plan_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  plan_version INT NOT NULL, fingerprint CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  base_snapshot_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, base_snapshot JSON NULL, target_snapshot JSON NOT NULL,
  target_snapshot_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, evidence_json JSON NOT NULL,
  action_text VARCHAR(255) NOT NULL, impact_text TEXT NOT NULL, risks_json JSON NOT NULL, observation_text TEXT NOT NULL,
  cooldown_until DATETIME(6) NULL, provenance VARCHAR(96) NOT NULL, status VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  version BIGINT NOT NULL, idempotency_key VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NULL,
  request_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL, accepted_change_set_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  created_by VARCHAR(96) NOT NULL, created_at DATETIME(6) NOT NULL, updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_delivery_recommendation_fingerprint (organization_id, project_id, fingerprint),
  UNIQUE KEY uq_delivery_recommendation_idempotency (organization_id, project_id, idempotency_key),
  KEY idx_delivery_recommendations_project (organization_id, project_id, created_at),
  CONSTRAINT chk_delivery_recommendation_status CHECK (status IN ('proposed','accepted','rejected'))
);
CREATE TABLE IF NOT EXISTS delivery_manual_action_packages (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  change_set_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, target_snapshot_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  content_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, package_json JSON NOT NULL, created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_delivery_manual_action_package (organization_id, project_id, change_set_id, target_snapshot_hash),
  KEY idx_delivery_manual_action_packages_project (organization_id, project_id, change_set_id)
);
