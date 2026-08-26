-- Operational Ocean Engine object catalog used by Delivery object pickers.
-- This catalog keeps exact platform IDs. The point-in-time ledger continues
-- to use opaque references for analytics and training boundaries.
CREATE TABLE connector_platform_objects (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  account_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  object_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  platform_object_id VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  display_name VARCHAR(512) NOT NULL DEFAULT '',
  status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'active',
  metadata_json JSON NOT NULL,
  source_fingerprint CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  last_sync_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  observed_at DATETIME(6) NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_connector_platform_object_organization (organization_id, id),
  UNIQUE KEY uq_connector_platform_object_scope (organization_id, account_id, object_kind, platform_object_id),
  KEY idx_connector_platform_object_list (organization_id, account_id, object_kind, status, id),
  CONSTRAINT fk_connector_platform_object_account FOREIGN KEY (organization_id, account_id) REFERENCES platform_accounts(organization_id, id),
  CONSTRAINT chk_connector_platform_object_kind CHECK (object_kind IN ('image_material', 'video_material', 'orange_landing_page')),
  CONSTRAINT chk_connector_platform_object_status CHECK (status IN ('active', 'unavailable'))
);

CREATE TABLE connector_platform_object_project_grants (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  platform_object_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'active',
  granted_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_connector_platform_object_grant (organization_id, project_id, platform_object_id),
  KEY idx_connector_platform_object_grant_object (organization_id, platform_object_id),
  CONSTRAINT fk_connector_platform_object_grant_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT fk_connector_platform_object_grant_object FOREIGN KEY (organization_id, platform_object_id) REFERENCES connector_platform_objects(organization_id, id),
  CONSTRAINT chk_connector_platform_object_grant_status CHECK (status IN ('active', 'revoked'))
);
