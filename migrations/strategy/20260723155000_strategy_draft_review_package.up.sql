CREATE TABLE strategy_drafts (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  task_id VARCHAR(96) NOT NULL,
  brief_id VARCHAR(96) NOT NULL,
  brief_version BIGINT NOT NULL,
  project_context_version BIGINT NOT NULL,
  status VARCHAR(32) NOT NULL,
  current_revision BIGINT NOT NULL DEFAULT 0,
  current_review_id VARCHAR(96) NULL,
  version BIGINT NOT NULL DEFAULT 1,
  skill_versions JSON NOT NULL,
  created_by VARCHAR(96) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_strategy_draft_scope (organization_id, project_id, id),
  KEY idx_strategy_draft_task (organization_id, project_id, task_id, created_at)
);

CREATE TABLE strategy_draft_revisions (
  strategy_id VARCHAR(96) NOT NULL,
  revision BIGINT NOT NULL,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  base_revision BIGINT NULL,
  document JSON NOT NULL,
  changed_sections JSON NOT NULL,
  content_hash VARCHAR(71) NOT NULL,
  lineage JSON NOT NULL,
  created_by VARCHAR(96) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (organization_id, project_id, strategy_id, revision),
  UNIQUE KEY uq_strategy_revision_hash (organization_id, project_id, strategy_id, content_hash)
);

CREATE TABLE strategy_reviews (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  strategy_id VARCHAR(96) NOT NULL,
  candidate_revision BIGINT NOT NULL,
  candidate_content_hash VARCHAR(71) NOT NULL,
  brief_id VARCHAR(96) NOT NULL,
  brief_version BIGINT NOT NULL,
  project_context_version BIGINT NOT NULL,
  status VARCHAR(24) NOT NULL,
  decision_reason TEXT NULL,
  decided_by VARCHAR(96) NULL,
  decided_at DATETIME(6) NULL,
  created_by VARCHAR(96) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_strategy_review_scope (organization_id, project_id, id),
  KEY idx_strategy_review_strategy (organization_id, project_id, strategy_id, created_at)
);

CREATE TABLE strategy_review_comments (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  review_id VARCHAR(96) NOT NULL,
  author_id VARCHAR(96) NOT NULL,
  body TEXT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  KEY idx_strategy_review_comment (organization_id, project_id, review_id, created_at)
);

CREATE TABLE strategy_packages (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  strategy_id VARCHAR(96) NOT NULL,
  latest_version BIGINT NOT NULL,
  status VARCHAR(24) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_strategy_package_scope (organization_id, project_id, id),
  UNIQUE KEY uq_strategy_package_series (organization_id, project_id, strategy_id)
);

CREATE TABLE strategy_package_versions (
  package_id VARCHAR(96) NOT NULL,
  version BIGINT NOT NULL,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  strategy_id VARCHAR(96) NOT NULL,
  strategy_revision BIGINT NOT NULL,
  review_id VARCHAR(96) NOT NULL,
  snapshot JSON NOT NULL,
  content_hash VARCHAR(71) NOT NULL,
  published_by VARCHAR(96) NOT NULL,
  published_at DATETIME(6) NOT NULL,
  status VARCHAR(24) NOT NULL,
  PRIMARY KEY (organization_id, project_id, package_id, version),
  UNIQUE KEY uq_strategy_package_hash (organization_id, project_id, package_id, content_hash)
);
