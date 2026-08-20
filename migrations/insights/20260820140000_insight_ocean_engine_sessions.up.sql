-- Project-scoped encrypted session material for the read-only Ocean Engine web API connector.
-- This table stores no plaintext Cookie, CSRF token, advertiser ID, or upstream response.
CREATE TABLE IF NOT EXISTS insight_ocean_engine_sessions (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'unverified',
  session_ciphertext VARBINARY(16384) NOT NULL,
  session_key_version VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  last_verified_at DATETIME(6) NULL,
  last_successful_request_at DATETIME(6) NULL,
  last_error_kind VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  last_error_code VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NULL,
  last_error_at DATETIME(6) NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,

  UNIQUE KEY uq_insight_ocean_engine_sessions_scope (organization_id, id),
  UNIQUE KEY uq_insight_ocean_engine_sessions_project (organization_id, project_id),
  KEY idx_insight_ocean_engine_sessions_project (organization_id, project_id, status, updated_at),
  CONSTRAINT fk_insight_ocean_engine_sessions_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT chk_insight_ocean_engine_sessions_status CHECK (status IN ('unverified', 'ready', 'auth_required', 'disabled')),
  CONSTRAINT chk_insight_ocean_engine_sessions_version CHECK (version > 0),
  CONSTRAINT chk_insight_ocean_engine_sessions_ciphertext CHECK (OCTET_LENGTH(session_ciphertext) > 0),
  CONSTRAINT chk_insight_ocean_engine_sessions_error_pair CHECK ((last_error_kind IS NULL AND last_error_code IS NULL) OR (last_error_kind IS NOT NULL AND last_error_code IS NOT NULL))
);
