CREATE TABLE strategy_briefs (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  latest_draft_id VARCHAR(96) NOT NULL,
  latest_version BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_strategy_brief_scope (organization_id, project_id, id)
);

CREATE TABLE strategy_brief_drafts (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  brief_id VARCHAR(96) NOT NULL,
  status VARCHAR(24) NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  base_brief_version BIGINT NULL,
  document JSON NOT NULL,
  field_states JSON NOT NULL,
  completeness JSON NOT NULL,
  updated_by VARCHAR(96) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_strategy_brief_draft_scope (organization_id, project_id, id),
  KEY idx_strategy_brief_draft_brief (organization_id, project_id, brief_id, created_at)
);

CREATE TABLE strategy_brief_revisions (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  draft_id VARCHAR(96) NOT NULL,
  draft_version BIGINT NOT NULL,
  patch JSON NOT NULL,
  snapshot_hash VARCHAR(71) NOT NULL,
  created_by VARCHAR(96) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_strategy_brief_revision (organization_id, project_id, draft_id, draft_version)
);

CREATE TABLE strategy_brief_versions (
  brief_id VARCHAR(96) NOT NULL,
  version BIGINT NOT NULL,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  snapshot JSON NOT NULL,
  content_hash VARCHAR(71) NOT NULL,
  source_draft_id VARCHAR(96) NOT NULL,
  source_draft_version BIGINT NOT NULL,
  confirmed_by VARCHAR(96) NOT NULL,
  confirmed_at DATETIME(6) NOT NULL,
  PRIMARY KEY (organization_id, project_id, brief_id, version),
  UNIQUE KEY uq_strategy_brief_version_hash (organization_id, project_id, brief_id, content_hash)
);
