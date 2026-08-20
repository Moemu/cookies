-- Unified account catalog for authorized platform connections.
-- Credential values are never stored; credential_ref points to a secret provider.
CREATE TABLE platform_accounts (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  platform VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  external_id VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  display_label VARCHAR(255) NOT NULL DEFAULT '',
  status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'pending',
  verified_at DATETIME(6) NULL,
  last_checked_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_platform_accounts_scope (organization_id, id),
  UNIQUE KEY uq_platform_accounts_external (organization_id, platform, external_id),
  CONSTRAINT chk_platform_accounts_platform CHECK (platform IN ('ocean_engine')),
  CONSTRAINT chk_platform_accounts_status CHECK (status IN ('pending', 'verified', 'revoked', 'blocked'))
);

CREATE TABLE platform_account_connections (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  account_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  connection_type VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  credential_ref VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'pending',
  last_verified_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_platform_connections_scope (organization_id, id),
  UNIQUE KEY uq_platform_connections_project_account (organization_id, project_id, account_id),
  CONSTRAINT fk_platform_connections_account FOREIGN KEY (organization_id, account_id) REFERENCES platform_accounts(organization_id, id),
  CONSTRAINT chk_platform_connections_type CHECK (connection_type IN ('web_api')),
  CONSTRAINT chk_platform_connections_status CHECK (status IN ('pending', 'verified', 'revoked', 'blocked'))
);
