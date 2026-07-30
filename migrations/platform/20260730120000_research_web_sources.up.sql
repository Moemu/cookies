ALTER TABLE platform_research_runs
  ADD COLUMN provider_code VARCHAR(32) NULL AFTER error_message,
  ADD COLUMN model_version VARCHAR(255) NULL AFTER provider_code,
  ADD COLUMN provider_response_id VARCHAR(255) NULL AFTER model_version,
  ADD COLUMN usage_json JSON NULL AFTER provider_response_id;

CREATE TABLE platform_research_sources (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  research_run_id VARCHAR(96) NOT NULL,
  source_class VARCHAR(24) NOT NULL,
  media_type VARCHAR(24) NOT NULL,
  title VARCHAR(512) NOT NULL,
  source_url VARCHAR(2048) NOT NULL,
  canonical_url VARCHAR(2048) NOT NULL,
  canonical_url_hash CHAR(64) NOT NULL,
  source_domain VARCHAR(255) NOT NULL,
  published_at DATETIME(6) NULL,
  retrieved_at DATETIME(6) NOT NULL,
  verification_status VARCHAR(24) NOT NULL,
  content_hash CHAR(64) NOT NULL,
  provider_metadata JSON NULL,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_research_source_scope (organization_id, project_id, id),
  UNIQUE KEY uq_research_source_url
    (organization_id, project_id, research_run_id, canonical_url_hash),
  KEY idx_research_source_run
    (organization_id, project_id, research_run_id, created_at),
  CONSTRAINT chk_research_source_verification
    CHECK (verification_status IN ('model_cited', 'content_verified', 'conflicted', 'invalid'))
);

CREATE TABLE platform_research_citations (
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  research_artifact_id VARCHAR(96) NOT NULL,
  research_source_id VARCHAR(96) NOT NULL,
  output_start INT NOT NULL DEFAULT -1,
  output_end INT NOT NULL DEFAULT -1,
  support_level VARCHAR(24) NOT NULL,
  provider_locator JSON NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (
    organization_id, project_id, research_artifact_id,
    research_source_id, output_start, output_end
  ),
  KEY idx_research_citation_source
    (organization_id, project_id, research_source_id),
  CONSTRAINT chk_research_citation_support
    CHECK (support_level IN ('model_cited', 'content_verified', 'conflicted', 'invalid'))
);
