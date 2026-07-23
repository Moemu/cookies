CREATE TABLE strategy_workspaces (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  name VARCHAR(255) NOT NULL,
  is_primary BOOLEAN NOT NULL DEFAULT FALSE,
  primary_slot TINYINT GENERATED ALWAYS AS (CASE WHEN is_primary = TRUE AND status = 'active' THEN 1 ELSE NULL END) STORED,
  status VARCHAR(24) NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_by VARCHAR(96) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_strategy_workspace_scope (organization_id, project_id, id),
  UNIQUE KEY uq_strategy_workspace_primary (organization_id, project_id, primary_slot),
  KEY idx_strategy_workspace_list (organization_id, project_id, status, created_at)
);

CREATE TABLE strategy_conversations (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  workspace_id VARCHAR(96) NOT NULL,
  status VARCHAR(24) NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_by VARCHAR(96) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_strategy_conversation_scope (organization_id, project_id, id),
  KEY idx_strategy_conversation_workspace (organization_id, project_id, workspace_id, created_at)
);

CREATE TABLE strategy_messages (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  conversation_id VARCHAR(96) NOT NULL,
  role VARCHAR(24) NOT NULL,
  content_type VARCHAR(32) NOT NULL,
  content TEXT NOT NULL,
  ai_generated BOOLEAN NOT NULL DEFAULT FALSE,
  agent_task_id VARCHAR(96) NULL,
  skill_run_ids JSON NULL,
  created_by VARCHAR(96) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_strategy_message_scope (organization_id, project_id, id),
  KEY idx_strategy_message_conversation (organization_id, project_id, conversation_id, created_at, id)
);

CREATE TABLE strategy_conversation_events (
  sequence BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  event_id VARCHAR(96) NOT NULL,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  conversation_id VARCHAR(96) NOT NULL,
  event_type VARCHAR(128) NOT NULL,
  payload JSON NOT NULL,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_strategy_conversation_event (organization_id, project_id, event_id),
  KEY idx_strategy_event_replay (organization_id, project_id, conversation_id, sequence)
);

CREATE TABLE strategy_tasks (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  workspace_id VARCHAR(96) NOT NULL,
  conversation_id VARCHAR(96) NOT NULL,
  brief_id VARCHAR(96) NOT NULL,
  current_agent_task_id VARCHAR(96) NULL,
  current_strategy_id VARCHAR(96) NULL,
  status VARCHAR(32) NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_by VARCHAR(96) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_strategy_task_scope (organization_id, project_id, id),
  KEY idx_strategy_task_workspace (organization_id, project_id, workspace_id, created_at)
);

CREATE TABLE strategy_write_receipts (
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  principal_kind VARCHAR(16) NOT NULL,
  principal_id VARCHAR(96) NOT NULL,
  operation_name VARCHAR(128) NOT NULL,
  idempotency_key VARCHAR(255) NOT NULL,
  request_hash CHAR(64) NOT NULL,
  response_status SMALLINT NOT NULL,
  response_snapshot JSON NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (organization_id, project_id, principal_kind, principal_id, operation_name, idempotency_key)
);
