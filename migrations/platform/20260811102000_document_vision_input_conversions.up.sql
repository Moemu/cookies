CREATE TABLE platform_knowledge_document_vision_input_conversions (
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  document_id VARCHAR(96) NOT NULL,
  attempt_id VARCHAR(96) NOT NULL,
  status VARCHAR(24) NOT NULL,
  attempt_count INT NOT NULL DEFAULT 0,
  source_mime_type VARCHAR(128) NOT NULL,
  source_sha256 CHAR(64) NOT NULL,
  source_provider VARCHAR(32) NOT NULL,
  source_bucket VARCHAR(255) NOT NULL,
  source_object_key VARCHAR(1024) NOT NULL,
  source_version_id VARCHAR(255) NOT NULL DEFAULT '',
  source_etag VARCHAR(255) NOT NULL DEFAULT '',
  converter_code VARCHAR(64) NOT NULL,
  converter_version VARCHAR(160) NOT NULL,
  derived_provider VARCHAR(32) NOT NULL DEFAULT '',
  derived_bucket VARCHAR(255) NOT NULL,
  derived_object_key VARCHAR(1024) NOT NULL,
  derived_version_id VARCHAR(255) NOT NULL DEFAULT '',
  derived_etag VARCHAR(255) NOT NULL DEFAULT '',
  derived_sha256 CHAR(64) NOT NULL DEFAULT '',
  derived_size_bytes BIGINT NULL,
  error_code VARCHAR(96) NOT NULL DEFAULT '',
  error_message VARCHAR(1024) NOT NULL DEFAULT '',
  started_at DATETIME(6) NULL,
  completed_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (organization_id, project_id, document_id, attempt_id),
  KEY idx_knowledge_document_vision_conversion_status
    (organization_id, project_id, status, updated_at),
  CONSTRAINT chk_knowledge_document_vision_conversion_status CHECK
    (status IN ('prepared', 'converting', 'ready', 'failed')),
  CONSTRAINT chk_knowledge_document_vision_conversion_size CHECK
    (derived_size_bytes IS NULL OR derived_size_bytes > 0),
  CONSTRAINT chk_knowledge_document_vision_conversion_attempts CHECK
    (attempt_count >= 0 AND attempt_count <= 3),
  CONSTRAINT fk_knowledge_document_vision_conversion_document
    FOREIGN KEY (organization_id, project_id, document_id)
    REFERENCES platform_knowledge_documents (organization_id, project_id, id)
    ON DELETE CASCADE
);
