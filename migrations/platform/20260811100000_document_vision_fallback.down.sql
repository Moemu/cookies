DROP TABLE IF EXISTS platform_knowledge_document_pages;

ALTER TABLE platform_knowledge_documents
  DROP INDEX idx_knowledge_document_vision_activity,
  DROP COLUMN vision_completed_at,
  DROP COLUMN vision_started_at,
  DROP COLUMN vision_error_message,
  DROP COLUMN vision_error_code,
  DROP COLUMN vision_model_version,
  DROP COLUMN vision_route_revision_id,
  DROP COLUMN vision_model_alias,
  DROP COLUMN vision_completed_pages,
  DROP COLUMN vision_selected_pages,
  DROP COLUMN vision_fallback_status;
