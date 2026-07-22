CREATE TABLE IF NOT EXISTS brands (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  name VARCHAR(255) NOT NULL,
  status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'active',
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_brands_org_id (organization_id, id),
  KEY idx_brands_org_status (organization_id, status),
  CONSTRAINT fk_brands_org FOREIGN KEY (organization_id) REFERENCES organizations(id),
  CONSTRAINT chk_brands_status CHECK (status IN ('active', 'archived'))
);

CREATE TABLE IF NOT EXISTS products (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  name VARCHAR(255) NOT NULL,
  status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'active',
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_products_org_id (organization_id, id),
  KEY idx_products_org_status (organization_id, status),
  CONSTRAINT fk_products_org FOREIGN KEY (organization_id) REFERENCES organizations(id),
  CONSTRAINT chk_products_status CHECK (status IN ('active', 'archived'))
);

CREATE TABLE IF NOT EXISTS brand_guideline_versions (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  brand_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  version BIGINT NOT NULL,
  status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'approved',
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_brand_guideline_version (organization_id, brand_id, version),
  UNIQUE KEY uq_brand_guideline_org_id (organization_id, id),
  CONSTRAINT fk_brand_guideline_brand FOREIGN KEY (organization_id, brand_id) REFERENCES brands(organization_id, id),
  CONSTRAINT chk_brand_guideline_version_positive CHECK (version > 0),
  CONSTRAINT chk_brand_guideline_status CHECK (status IN ('draft', 'approved', 'archived'))
);

CREATE TABLE IF NOT EXISTS projects (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  name VARCHAR(255) NOT NULL,
  status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'draft',
  primary_brand_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  brand_guideline_version_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  project_context_version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_projects_org_id (organization_id, id),
  KEY idx_projects_org_status (organization_id, status, updated_at),
  CONSTRAINT fk_projects_org FOREIGN KEY (organization_id) REFERENCES organizations(id),
  CONSTRAINT fk_projects_brand FOREIGN KEY (organization_id, primary_brand_id) REFERENCES brands(organization_id, id),
  CONSTRAINT fk_projects_guideline FOREIGN KEY (organization_id, brand_guideline_version_id) REFERENCES brand_guideline_versions(organization_id, id),
  CONSTRAINT chk_projects_status CHECK (status IN ('draft', 'active', 'archived')),
  CONSTRAINT chk_projects_context_version CHECK (project_context_version > 0),
  CONSTRAINT chk_projects_active_brand CHECK (status <> 'active' OR primary_brand_id IS NOT NULL)
);

CREATE TABLE IF NOT EXISTS project_memberships (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  principal_kind VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  principal_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  role VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'active',
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (organization_id, project_id, principal_kind, principal_id),
  KEY idx_project_memberships_principal (organization_id, principal_kind, principal_id, status),
  CONSTRAINT fk_project_memberships_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT chk_project_memberships_kind CHECK (principal_kind IN ('user', 'service')),
  CONSTRAINT chk_project_memberships_role CHECK (role IN ('owner', 'editor', 'viewer', 'worker')),
  CONSTRAINT chk_project_memberships_status CHECK (status IN ('active', 'suspended', 'removed'))
);

CREATE TABLE IF NOT EXISTS project_products (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  product_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (organization_id, project_id, product_id),
  CONSTRAINT fk_project_products_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT fk_project_products_product FOREIGN KEY (organization_id, product_id) REFERENCES products(organization_id, id)
);

CREATE TABLE IF NOT EXISTS project_context_versions (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  version BIGINT NOT NULL,
  brand_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  brand_guideline_version_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  product_ids JSON NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (organization_id, project_id, version),
  CONSTRAINT fk_project_context_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT fk_project_context_brand FOREIGN KEY (organization_id, brand_id) REFERENCES brands(organization_id, id),
  CONSTRAINT fk_project_context_guideline FOREIGN KEY (organization_id, brand_guideline_version_id) REFERENCES brand_guideline_versions(organization_id, id),
  CONSTRAINT chk_project_context_version_positive CHECK (version > 0)
);
