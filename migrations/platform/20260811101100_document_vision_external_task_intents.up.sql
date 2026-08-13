-- Forward repair for schemas that applied the first document-vision task
-- migration before durable submission intents and reconciliation were added.
-- Every statement is conditional so a fresh schema that already has the
-- complete preceding migration remains a no-op.

SET @cookies_ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE()
    AND table_name = 'platform_knowledge_document_vision_tasks' AND column_name = 'intent_id'),
  'SELECT 1',
  'ALTER TABLE platform_knowledge_document_vision_tasks ADD COLUMN intent_id CHAR(64) NOT NULL DEFAULT '''' AFTER status'
);
PREPARE cookies_stmt FROM @cookies_ddl;
EXECUTE cookies_stmt;
DEALLOCATE PREPARE cookies_stmt;

SET @cookies_ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE()
    AND table_name = 'platform_knowledge_document_vision_tasks' AND column_name = 'external_task_binding'),
  'SELECT 1',
  'ALTER TABLE platform_knowledge_document_vision_tasks ADD COLUMN external_task_binding VARCHAR(160) GENERATED ALWAYS AS (NULLIF(external_task_id, '''')) STORED AFTER external_task_id'
);
PREPARE cookies_stmt FROM @cookies_ddl;
EXECUTE cookies_stmt;
DEALLOCATE PREPARE cookies_stmt;

SET @cookies_ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE()
    AND table_name = 'platform_knowledge_document_vision_tasks'
    AND index_name = 'uq_knowledge_document_vision_external_binding'),
  'SELECT 1',
  'ALTER TABLE platform_knowledge_document_vision_tasks ADD UNIQUE KEY uq_knowledge_document_vision_external_binding (organization_id, external_task_binding)'
);
PREPARE cookies_stmt FROM @cookies_ddl;
EXECUTE cookies_stmt;
DEALLOCATE PREPARE cookies_stmt;

SET @cookies_ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE()
    AND table_name = 'platform_knowledge_document_vision_tasks'
    AND index_name = 'idx_knowledge_document_vision_intent'),
  'SELECT 1',
  'ALTER TABLE platform_knowledge_document_vision_tasks ADD KEY idx_knowledge_document_vision_intent (organization_id, intent_id)'
);
PREPARE cookies_stmt FROM @cookies_ddl;
EXECUTE cookies_stmt;
DEALLOCATE PREPARE cookies_stmt;

SET @cookies_ddl = CASE
  WHEN NOT EXISTS(SELECT 1 FROM information_schema.table_constraints
    WHERE constraint_schema = DATABASE() AND table_name = 'platform_knowledge_document_vision_tasks'
      AND constraint_name = 'chk_knowledge_document_vision_task_status')
    THEN 'ALTER TABLE platform_knowledge_document_vision_tasks ADD CONSTRAINT chk_knowledge_document_vision_task_status CHECK (status IN (''prepared'', ''submitting'', ''submitted'', ''running'', ''completed'', ''failed'', ''unknown''))'
  WHEN EXISTS(SELECT 1 FROM information_schema.check_constraints
    WHERE constraint_schema = DATABASE() AND constraint_name = 'chk_knowledge_document_vision_task_status'
      AND check_clause LIKE '%submitting%')
    THEN 'SELECT 1'
  ELSE 'ALTER TABLE platform_knowledge_document_vision_tasks DROP CHECK chk_knowledge_document_vision_task_status, ADD CONSTRAINT chk_knowledge_document_vision_task_status CHECK (status IN (''prepared'', ''submitting'', ''submitted'', ''running'', ''completed'', ''failed'', ''unknown''))'
END;
PREPARE cookies_stmt FROM @cookies_ddl;
EXECUTE cookies_stmt;
DEALLOCATE PREPARE cookies_stmt;

SET @cookies_ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.table_constraints
    WHERE constraint_schema = DATABASE() AND table_name = 'platform_knowledge_document_vision_tasks'
      AND constraint_name = 'chk_knowledge_document_vision_task_intent'),
  'SELECT 1',
  'ALTER TABLE platform_knowledge_document_vision_tasks ADD CONSTRAINT chk_knowledge_document_vision_task_intent CHECK (intent_id = '''' OR intent_id REGEXP ''^[a-f0-9]{64}$'')'
);
PREPARE cookies_stmt FROM @cookies_ddl;
EXECUTE cookies_stmt;
DEALLOCATE PREPARE cookies_stmt;

CREATE TABLE IF NOT EXISTS platform_knowledge_document_vision_reconciliations (
  id VARCHAR(96) NOT NULL,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  document_id VARCHAR(96) NOT NULL,
  attempt_id VARCHAR(96) NOT NULL,
  task_index INT NOT NULL,
  intent_id CHAR(64) NOT NULL,
  decision VARCHAR(24) NOT NULL,
  external_task_id VARCHAR(160) NOT NULL DEFAULT '',
  external_task_binding VARCHAR(160)
    GENERATED ALWAYS AS (NULLIF(external_task_id, '')) STORED,
  evidence_ref VARCHAR(512) NOT NULL,
  status VARCHAR(24) NOT NULL,
  proposed_by VARCHAR(96) NOT NULL,
  proposed_at DATETIME(6) NOT NULL,
  confirmed_by VARCHAR(96) NOT NULL DEFAULT '',
  confirmed_at DATETIME(6) NULL,
  applied_at DATETIME(6) NULL,
  scheduled_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (organization_id, project_id, id),
  KEY idx_knowledge_document_vision_reconciliation_task
    (organization_id, project_id, document_id, attempt_id, task_index, status, created_at),
  KEY idx_knowledge_document_vision_reconciliation_schedule
    (status, decision, scheduled_at, updated_at),
  UNIQUE KEY uq_knowledge_document_vision_reconciliation_external_binding
    (organization_id, external_task_binding),
  CONSTRAINT chk_knowledge_document_vision_reconciliation_intent CHECK
    (intent_id REGEXP '^[a-f0-9]{64}$'),
  CONSTRAINT chk_knowledge_document_vision_reconciliation_decision CHECK
    (decision IN ('accepted', 'not_accepted')),
  CONSTRAINT chk_knowledge_document_vision_reconciliation_status CHECK
    (status IN ('proposed', 'rejected', 'applied')),
  CONSTRAINT chk_knowledge_document_vision_reconciliation_external CHECK
    ((decision = 'accepted' AND external_task_id <> '') OR
     (decision = 'not_accepted' AND external_task_id = '')),
  CONSTRAINT chk_knowledge_document_vision_reconciliation_confirmed CHECK
    ((status = 'proposed' AND confirmed_by = '' AND confirmed_at IS NULL AND applied_at IS NULL) OR
     (status = 'rejected' AND confirmed_by <> '' AND confirmed_at IS NOT NULL AND applied_at IS NULL) OR
     (status = 'applied' AND confirmed_by <> '' AND confirmed_at IS NOT NULL AND applied_at IS NOT NULL)),
  CONSTRAINT chk_knowledge_document_vision_reconciliation_two_actors CHECK
    (confirmed_by = '' OR confirmed_by <> proposed_by),
  CONSTRAINT fk_knowledge_document_vision_reconciliation_task
    FOREIGN KEY (organization_id, project_id, document_id, attempt_id, task_index)
    REFERENCES platform_knowledge_document_vision_tasks
      (organization_id, project_id, document_id, attempt_id, task_index)
    ON DELETE CASCADE
);
