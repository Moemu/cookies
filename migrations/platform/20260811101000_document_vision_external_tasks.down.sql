DROP TABLE IF EXISTS platform_knowledge_document_vision_reconciliations;
DROP TABLE IF EXISTS platform_knowledge_document_vision_tasks;

ALTER TABLE platform_knowledge_documents
  DROP COLUMN vision_attempt_id;
