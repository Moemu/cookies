CREATE TABLE IF NOT EXISTS organizations (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'active',
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  CONSTRAINT chk_organizations_status CHECK (status IN ('active', 'suspended', 'archived'))
);

CREATE TABLE IF NOT EXISTS users (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  display_name VARCHAR(255) NOT NULL,
  status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'active',
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  CONSTRAINT chk_users_status CHECK (status IN ('active', 'suspended', 'deleted'))
);

CREATE TABLE IF NOT EXISTS external_identities (
  provider VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  external_subject VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  user_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (provider, external_subject),
  KEY idx_external_identities_user (user_id),
  CONSTRAINT fk_external_identities_user FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS organization_memberships (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  user_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  role VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'active',
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (organization_id, user_id),
  KEY idx_organization_memberships_user (user_id, status),
  CONSTRAINT fk_organization_memberships_org FOREIGN KEY (organization_id) REFERENCES organizations(id),
  CONSTRAINT fk_organization_memberships_user FOREIGN KEY (user_id) REFERENCES users(id),
  CONSTRAINT chk_organization_memberships_role CHECK (role IN ('owner', 'admin', 'member', 'auditor')),
  CONSTRAINT chk_organization_memberships_status CHECK (status IN ('active', 'suspended', 'removed'))
);

CREATE TABLE IF NOT EXISTS service_identities (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  name VARCHAR(255) NOT NULL,
  status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'active',
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_service_identities_org_id (organization_id, id),
  KEY idx_service_identities_org_status (organization_id, status),
  CONSTRAINT fk_service_identities_org FOREIGN KEY (organization_id) REFERENCES organizations(id),
  CONSTRAINT chk_service_identities_status CHECK (status IN ('active', 'suspended', 'revoked'))
);

CREATE TABLE IF NOT EXISTS service_identity_scopes (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  service_identity_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  scope VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (organization_id, service_identity_id, scope),
  CONSTRAINT fk_service_identity_scopes_identity FOREIGN KEY (organization_id, service_identity_id)
    REFERENCES service_identities(organization_id, id)
);
