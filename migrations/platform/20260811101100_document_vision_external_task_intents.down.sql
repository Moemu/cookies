DROP TABLE IF EXISTS platform_knowledge_document_vision_reconciliations;

ALTER TABLE platform_knowledge_document_vision_tasks
  DROP CHECK chk_knowledge_document_vision_task_intent,
  DROP CHECK chk_knowledge_document_vision_task_status,
  ADD CONSTRAINT chk_knowledge_document_vision_task_status CHECK
    (status IN ('prepared', 'submitted', 'running', 'completed', 'failed', 'unknown')),
  DROP INDEX idx_knowledge_document_vision_intent,
  DROP INDEX uq_knowledge_document_vision_external_binding,
  DROP COLUMN external_task_binding,
  DROP COLUMN intent_id;
