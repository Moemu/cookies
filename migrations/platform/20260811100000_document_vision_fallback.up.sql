ALTER TABLE platform_knowledge_documents
  ADD COLUMN vision_fallback_status VARCHAR(24) NOT NULL DEFAULT 'not_requested' AFTER heartbeat_at,
  ADD COLUMN vision_selected_pages JSON NULL AFTER vision_fallback_status,
  ADD COLUMN vision_completed_pages JSON NULL AFTER vision_selected_pages,
  ADD COLUMN vision_model_alias VARCHAR(160) NOT NULL DEFAULT '' AFTER vision_completed_pages,
  ADD COLUMN vision_route_revision_id VARCHAR(96) NOT NULL DEFAULT '' AFTER vision_model_alias,
  ADD COLUMN vision_model_version VARCHAR(160) NOT NULL DEFAULT '' AFTER vision_route_revision_id,
  ADD COLUMN vision_error_code VARCHAR(96) NOT NULL DEFAULT '' AFTER vision_model_version,
  ADD COLUMN vision_error_message VARCHAR(1024) NOT NULL DEFAULT '' AFTER vision_error_code,
  ADD COLUMN vision_started_at DATETIME(6) NULL AFTER vision_error_message,
  ADD COLUMN vision_completed_at DATETIME(6) NULL AFTER vision_started_at,
  ADD INDEX idx_knowledge_document_vision_activity
    (organization_id, project_id, vision_fallback_status, heartbeat_at, updated_at);

CREATE TABLE platform_knowledge_document_pages (
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  document_id VARCHAR(96) NOT NULL,
  page_number INT NOT NULL,
  selection_reason VARCHAR(64) NOT NULL,
  status VARCHAR(24) NOT NULL,
  text_origin VARCHAR(24) NOT NULL,
  vision_text MEDIUMTEXT NULL,
  merged_text MEDIUMTEXT NULL,
  text_sha256 CHAR(64) NOT NULL DEFAULT '',
  locator_json JSON NOT NULL,
  provider_code VARCHAR(64) NOT NULL DEFAULT '',
  model_alias VARCHAR(160) NOT NULL DEFAULT '',
  model_version VARCHAR(160) NOT NULL DEFAULT '',
  route_revision_id VARCHAR(96) NOT NULL DEFAULT '',
  latency_ms BIGINT NULL,
  usage_json JSON NULL,
  error_code VARCHAR(96) NOT NULL DEFAULT '',
  error_message VARCHAR(1024) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (organization_id, project_id, document_id, page_number),
  KEY idx_knowledge_document_pages_status
    (organization_id, project_id, document_id, status, page_number),
  CONSTRAINT chk_knowledge_document_page_number CHECK (page_number > 0),
  CONSTRAINT chk_knowledge_document_page_status
    CHECK (status IN ('selected', 'processing', 'ready', 'failed')),
  CONSTRAINT chk_knowledge_document_page_origin
    CHECK (text_origin IN ('unknown', 'tika', 'vision', 'hybrid')),
  CONSTRAINT fk_knowledge_document_pages_document
    FOREIGN KEY (organization_id, project_id, document_id)
    REFERENCES platform_knowledge_documents (organization_id, project_id, id)
    ON DELETE CASCADE
);
