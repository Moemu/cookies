-- Organization-scoped encrypted session material for Connector accounts.
-- Business Projects and Plans are optional consumers, not account owners.
CREATE TABLE connector_ocean_engine_account_sessions (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  account_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'unverified',
  session_ciphertext VARBINARY(16384) NOT NULL,
  session_key_version VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  last_verified_at DATETIME(6) NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_connector_account_session_scope (organization_id, account_id),
  CONSTRAINT fk_connector_account_session_account FOREIGN KEY (organization_id, account_id) REFERENCES platform_accounts(organization_id, id),
  CONSTRAINT chk_connector_account_session_status CHECK (status IN ('unverified', 'ready', 'auth_required', 'disabled')),
  CONSTRAINT chk_connector_account_session_version CHECK (version > 0),
  CONSTRAINT chk_connector_account_session_ciphertext CHECK (OCTET_LENGTH(session_ciphertext) > 0)
);
