CREATE TABLE IF NOT EXISTS identity_password_credentials (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  user_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  username VARCHAR(128) NOT NULL,
  username_normalized VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  password_hash VARBINARY(255) NOT NULL,
  failed_attempts INT NOT NULL DEFAULT 0,
  locked_until DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (organization_id, user_id),
  UNIQUE KEY uq_identity_password_username (username_normalized),
  CONSTRAINT fk_identity_password_membership FOREIGN KEY (organization_id, user_id)
    REFERENCES organization_memberships(organization_id, user_id),
  CONSTRAINT chk_identity_password_failed_attempts CHECK (failed_attempts >= 0)
);

CREATE TABLE IF NOT EXISTS identity_sessions (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  user_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  token_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  scopes JSON NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  revoked_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  last_seen_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_identity_session_token_hash (token_hash),
  KEY idx_identity_session_expiry (expires_at, revoked_at),
  CONSTRAINT fk_identity_session_membership FOREIGN KEY (organization_id, user_id)
    REFERENCES organization_memberships(organization_id, user_id)
);
