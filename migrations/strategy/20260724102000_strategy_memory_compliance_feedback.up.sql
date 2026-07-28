CREATE TABLE strategy_conversation_memories (
  conversation_id VARCHAR(96) NOT NULL,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  summary MEDIUMTEXT NOT NULL,
  open_questions JSON NOT NULL,
  last_message_id VARCHAR(96) NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (organization_id, project_id, conversation_id)
);

CREATE TABLE strategy_compliance_reports (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  strategy_id VARCHAR(96) NOT NULL,
  strategy_revision BIGINT NOT NULL,
  candidate_content_hash VARCHAR(80) NOT NULL,
  passed BOOLEAN NOT NULL,
  report JSON NOT NULL,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_strategy_compliance_revision
    (organization_id, project_id, strategy_id, strategy_revision),
  KEY idx_strategy_compliance_hash
    (organization_id, project_id, strategy_id, candidate_content_hash)
);

CREATE TABLE strategy_feedback (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  target_type VARCHAR(32) NOT NULL,
  target_id VARCHAR(96) NOT NULL,
  target_version BIGINT NOT NULL,
  rating VARCHAR(24) NOT NULL,
  comment VARCHAR(2000) NULL,
  created_by VARCHAR(96) NOT NULL,
  idempotency_key VARCHAR(128) NOT NULL,
  request_hash CHAR(64) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_strategy_feedback_idempotency
    (organization_id, project_id, created_by, idempotency_key),
  KEY idx_strategy_feedback_target
    (organization_id, project_id, target_type, target_id, target_version, created_at)
);
