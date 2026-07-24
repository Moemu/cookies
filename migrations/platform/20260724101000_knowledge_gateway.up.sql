CREATE TABLE platform_knowledge_documents (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  filename VARCHAR(512) NOT NULL,
  mime_type VARCHAR(128) NOT NULL,
  size_bytes BIGINT NOT NULL,
  content_sha256 CHAR(64) NOT NULL,
  text_sha256 CHAR(64) NOT NULL,
  extracted_text MEDIUMTEXT NOT NULL,
  object_provider VARCHAR(32) NOT NULL,
  object_bucket VARCHAR(255) NOT NULL,
  object_key VARCHAR(1024) NOT NULL,
  object_version_id VARCHAR(255) NULL,
  object_etag VARCHAR(255) NULL,
  status VARCHAR(24) NOT NULL,
  created_by VARCHAR(96) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_knowledge_document_scope (organization_id, project_id, id),
  KEY idx_knowledge_document_project (organization_id, project_id, created_at)
);

CREATE TABLE platform_research_runs (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  mode VARCHAR(16) NOT NULL,
  query_text VARCHAR(2000) NOT NULL,
  document_ids JSON NOT NULL,
  disclosed_fields JSON NOT NULL,
  status VARCHAR(24) NOT NULL,
  confirmed_by VARCHAR(96) NOT NULL,
  confirmed_at DATETIME(6) NOT NULL,
  error_code VARCHAR(128) NULL,
  error_message VARCHAR(1024) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_research_run_scope (organization_id, project_id, id),
  KEY idx_research_run_project (organization_id, project_id, created_at)
);

CREATE TABLE platform_research_artifacts (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  research_run_id VARCHAR(96) NOT NULL,
  source_type VARCHAR(32) NOT NULL,
  title VARCHAR(512) NOT NULL,
  source_url VARCHAR(2048) NULL,
  content MEDIUMTEXT NOT NULL,
  citations JSON NOT NULL,
  content_hash CHAR(64) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_research_artifact_scope (organization_id, project_id, id),
  KEY idx_research_artifact_run (organization_id, project_id, research_run_id)
);
