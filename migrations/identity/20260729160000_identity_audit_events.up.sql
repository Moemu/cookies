CREATE TABLE IF NOT EXISTS identity_audit_events (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  actor_user_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  target_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  target_id VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  action VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  before_state JSON NULL,
  after_state JSON NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  KEY idx_identity_audit_org_created (organization_id, created_at),
  KEY idx_identity_audit_target (organization_id, target_kind, target_id, created_at),
  CONSTRAINT fk_identity_audit_org FOREIGN KEY (organization_id) REFERENCES organizations(id),
  CONSTRAINT fk_identity_audit_actor FOREIGN KEY (actor_user_id) REFERENCES users(id),
  CONSTRAINT chk_identity_audit_before_json CHECK (before_state IS NULL OR JSON_VALID(before_state)),
  CONSTRAINT chk_identity_audit_after_json CHECK (after_state IS NULL OR JSON_VALID(after_state))
);
