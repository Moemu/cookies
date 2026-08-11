CREATE TABLE strategy_product_events (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  workspace_id VARCHAR(96) NULL,
  event_type VARCHAR(96) NOT NULL,
  stage VARCHAR(24) NULL,
  actor_kind VARCHAR(16) NOT NULL,
  actor_id_hash CHAR(64) NOT NULL,
  resource_type VARCHAR(48) NULL,
  resource_id VARCHAR(96) NULL,
  resource_version BIGINT NULL,
  duration_ms BIGINT NULL,
  outcome VARCHAR(24) NULL,
  attributes_json JSON NOT NULL,
  occurred_at DATETIME(6) NOT NULL,
  CONSTRAINT chk_strategy_product_event_stage CHECK (
    stage IS NULL OR stage IN ('intake', 'brief', 'strategy', 'review', 'handoff')
  ),
  CONSTRAINT chk_strategy_product_event_actor CHECK (
    actor_kind IN ('user', 'service')
  ),
  CONSTRAINT chk_strategy_product_event_resource CHECK (
    (resource_type IS NULL AND resource_id IS NULL AND resource_version IS NULL)
    OR (resource_type IS NOT NULL AND resource_id IS NOT NULL AND (resource_version IS NULL OR resource_version > 0))
  ),
  CONSTRAINT chk_strategy_product_event_duration CHECK (
    duration_ms IS NULL OR duration_ms >= 0
  ),
  CONSTRAINT chk_strategy_product_event_outcome CHECK (
    outcome IS NULL OR outcome IN (
      'accepted', 'viewed', 'succeeded', 'partial', 'failed', 'cancelled',
      'stalled', 'retried', 'edited', 'ignored', 'stale', 'approved', 'returned'
    )
  ),
  KEY idx_strategy_product_event_project_time
    (organization_id, project_id, occurred_at, id),
  KEY idx_strategy_product_event_workspace_stage
    (organization_id, project_id, workspace_id, stage, occurred_at),
  KEY idx_strategy_product_event_type_outcome
    (organization_id, project_id, event_type, outcome, occurred_at)
);
