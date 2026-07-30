ALTER TABLE platform_knowledge_documents
  ADD COLUMN parser_code VARCHAR(32) NULL AFTER status,
  ADD COLUMN parser_version VARCHAR(64) NULL AFTER parser_code,
  ADD COLUMN parse_error_code VARCHAR(128) NULL AFTER parser_version,
  ADD COLUMN parse_error_message VARCHAR(1024) NULL AFTER parse_error_code,
  ADD COLUMN parse_metadata JSON NULL AFTER parse_error_message,
  ADD COLUMN parsed_at DATETIME(6) NULL AFTER parse_metadata;

CREATE TABLE platform_knowledge_chunks (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  document_id VARCHAR(96) NOT NULL,
  ordinal INT NOT NULL,
  kind VARCHAR(24) NOT NULL,
  section VARCHAR(512) NOT NULL,
  page_number INT NULL,
  start_line INT NOT NULL,
  end_line INT NOT NULL,
  text MEDIUMTEXT NOT NULL,
  text_sha256 CHAR(64) NOT NULL,
  locator_json JSON NOT NULL,
  parser_code VARCHAR(32) NOT NULL,
  parser_version VARCHAR(64) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_knowledge_chunk_scope (organization_id, project_id, id),
  UNIQUE KEY uq_knowledge_chunk_ordinal
    (organization_id, project_id, document_id, ordinal),
  KEY idx_knowledge_chunk_document
    (organization_id, project_id, document_id, ordinal)
);

ALTER TABLE platform_research_runs
  ADD COLUMN disclosed_chunk_ids JSON NULL AFTER disclosed_fields;

UPDATE platform_research_runs
SET disclosed_chunk_ids = JSON_ARRAY()
WHERE disclosed_chunk_ids IS NULL;

ALTER TABLE platform_research_runs
  MODIFY COLUMN disclosed_chunk_ids JSON NOT NULL;
