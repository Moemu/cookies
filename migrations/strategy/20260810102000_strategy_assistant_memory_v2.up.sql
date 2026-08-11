ALTER TABLE strategy_conversation_memories
  ADD COLUMN summary_kind VARCHAR(24) NOT NULL DEFAULT 'deterministic' AFTER summary,
  ADD COLUMN summary_model_alias VARCHAR(96) NULL AFTER summary_kind,
  ADD COLUMN summary_prompt_version VARCHAR(96) NULL AFTER summary_model_alias,
  ADD COLUMN summary_content_hash VARCHAR(80) NOT NULL DEFAULT '' AFTER summary_prompt_version,
  ADD COLUMN recent_window_start_message_id VARCHAR(96) NULL AFTER last_message_id,
  ADD COLUMN artifact_manifest_json JSON NULL AFTER recent_window_start_message_id,
  ADD COLUMN last_compacted_at DATETIME(6) NULL AFTER artifact_manifest_json;

ALTER TABLE strategy_conversation_memories
  ADD CONSTRAINT chk_strategy_memory_summary_kind
  CHECK (summary_kind IN ('deterministic', 'model'));

CREATE TABLE strategy_artifact_proposals (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  workspace_id VARCHAR(96) NOT NULL,
  conversation_id VARCHAR(96) NOT NULL,
  proposal_kind VARCHAR(24) NOT NULL,
  target_type VARCHAR(32) NOT NULL,
  target_id VARCHAR(96) NOT NULL,
  target_version BIGINT NOT NULL,
  base_content_hash VARCHAR(80) NOT NULL,
  operations JSON NOT NULL,
  rationale VARCHAR(2000) NOT NULL,
  risk VARCHAR(16) NOT NULL,
  status VARCHAR(24) NOT NULL,
  source_message_id VARCHAR(96) NULL,
  created_by VARCHAR(96) NOT NULL,
  applied_by VARCHAR(96) NULL,
  applied_at DATETIME(6) NULL,
  ignored_by VARCHAR(96) NULL,
  ignored_at DATETIME(6) NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  KEY idx_strategy_artifact_proposals_scope
    (organization_id, project_id, workspace_id, status, created_at),
  KEY idx_strategy_artifact_proposals_conversation
    (organization_id, project_id, conversation_id, created_at),
  CONSTRAINT chk_strategy_artifact_proposal_kind
    CHECK (proposal_kind IN ('assistant', 'research')),
  CONSTRAINT chk_strategy_artifact_proposal_target
    CHECK (target_type IN ('brief_draft', 'strategy_revision')),
  CONSTRAINT chk_strategy_artifact_proposal_risk
    CHECK (risk IN ('low', 'medium', 'high')),
  CONSTRAINT chk_strategy_artifact_proposal_status
    CHECK (status IN ('proposed', 'applied', 'edited', 'ignored', 'stale'))
);
