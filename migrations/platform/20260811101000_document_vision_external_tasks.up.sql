ALTER TABLE platform_knowledge_documents
  ADD COLUMN vision_attempt_id VARCHAR(96) NOT NULL DEFAULT '' AFTER vision_fallback_status;

CREATE TABLE platform_knowledge_document_vision_tasks (
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  document_id VARCHAR(96) NOT NULL,
  attempt_id VARCHAR(96) NOT NULL,
  task_index INT NOT NULL,
  page_numbers JSON NOT NULL,
  status VARCHAR(24) NOT NULL,
  intent_id CHAR(64) NOT NULL DEFAULT '',
  external_task_id VARCHAR(160) NOT NULL DEFAULT '',
  external_task_binding VARCHAR(160)
    GENERATED ALWAYS AS (NULLIF(external_task_id, '')) STORED,
  provider_code VARCHAR(64) NOT NULL DEFAULT '',
  model_alias VARCHAR(160) NOT NULL DEFAULT '',
  model_version VARCHAR(160) NOT NULL DEFAULT '',
  route_revision_id VARCHAR(96) NOT NULL DEFAULT '',
  checkpoint_json JSON NULL,
  billable_pages INT NULL,
  error_code VARCHAR(96) NOT NULL DEFAULT '',
  error_message VARCHAR(1024) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (organization_id, project_id, document_id, attempt_id, task_index),
  KEY idx_knowledge_document_vision_external_task
    (organization_id, external_task_id),
  UNIQUE KEY uq_knowledge_document_vision_external_binding
    (organization_id, external_task_binding),
  KEY idx_knowledge_document_vision_intent
    (organization_id, intent_id),
  KEY idx_knowledge_document_vision_task_status
    (organization_id, project_id, document_id, attempt_id, status, task_index),
  CONSTRAINT chk_knowledge_document_vision_task_index CHECK (task_index >= 0),
  CONSTRAINT chk_knowledge_document_vision_task_status CHECK
    (status IN ('prepared', 'submitting', 'submitted', 'running', 'completed', 'failed', 'unknown')),
  CONSTRAINT chk_knowledge_document_vision_task_billable_pages CHECK
    (billable_pages IS NULL OR billable_pages >= 0),
  CONSTRAINT chk_knowledge_document_vision_task_intent CHECK
    (intent_id = '' OR intent_id REGEXP '^[a-f0-9]{64}$'),
  CONSTRAINT fk_knowledge_document_vision_task_document
    FOREIGN KEY (organization_id, project_id, document_id)
    REFERENCES platform_knowledge_documents (organization_id, project_id, id)
    ON DELETE CASCADE
);

CREATE TABLE platform_knowledge_document_vision_reconciliations (
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
