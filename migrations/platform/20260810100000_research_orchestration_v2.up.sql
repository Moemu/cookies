ALTER TABLE platform_research_runs
  ADD COLUMN contract_version VARCHAR(64) NOT NULL DEFAULT 'strategy-research-run/v2' AFTER id,
  ADD COLUMN run_mode VARCHAR(16) NOT NULL DEFAULT 'quick' AFTER mode,
  ADD COLUMN current_round INT NOT NULL DEFAULT 0 AFTER status,
  ADD COLUMN max_rounds INT NOT NULL DEFAULT 1 AFTER current_round,
  ADD COLUMN time_budget_seconds INT NOT NULL DEFAULT 120 AFTER max_rounds,
  ADD COLUMN token_budget BIGINT NOT NULL DEFAULT 12000 AFTER time_budget_seconds,
  ADD COLUMN input_snapshot_ref VARCHAR(160) NOT NULL DEFAULT 'legacy:unavailable' AFTER token_budget,
  ADD COLUMN input_snapshot_hash VARCHAR(80) NOT NULL DEFAULT '' AFTER input_snapshot_ref,
  ADD COLUMN input_snapshot_json JSON NULL AFTER input_snapshot_hash,
  ADD COLUMN coverage_json JSON NOT NULL AFTER input_snapshot_json,
  ADD COLUMN open_gaps_json JSON NOT NULL AFTER coverage_json,
  ADD COLUMN stop_reason VARCHAR(96) NOT NULL DEFAULT '' AFTER open_gaps_json,
  ADD COLUMN heartbeat_at DATETIME(6) NULL AFTER stop_reason,
  ADD COLUMN report_artifact_id VARCHAR(96) NULL AFTER heartbeat_at,
  ADD COLUMN started_at DATETIME(6) NULL AFTER report_artifact_id,
  ADD COLUMN completed_at DATETIME(6) NULL AFTER started_at;

UPDATE platform_research_runs
SET run_mode = CASE WHEN purpose = 'deep_research' THEN 'deep' ELSE 'quick' END,
    max_rounds = CASE WHEN purpose = 'deep_research' THEN 6 ELSE 1 END,
    time_budget_seconds = CASE WHEN purpose = 'deep_research' THEN 900 ELSE 120 END,
    token_budget = CASE WHEN purpose = 'deep_research' THEN 72000 ELSE 12000 END,
    input_snapshot_ref = CASE
      WHEN source_type IS NOT NULL AND source_id IS NOT NULL
        THEN CONCAT(source_type, ':', source_id)
      ELSE CONCAT('legacy:', id)
    END,
    input_snapshot_hash = CONCAT('sha256:', SHA2(CONCAT(
      id, '|', project_id, '|', query_text, '|', CAST(document_ids AS CHAR), '|', CAST(disclosed_fields AS CHAR)
    ), 256)),
    coverage_json = JSON_OBJECT(),
    open_gaps_json = JSON_ARRAY(),
    stop_reason = CASE
      WHEN status = 'succeeded' AND purpose = 'deep_research' THEN 'legacy_run_without_findings'
      WHEN status = 'succeeded' THEN 'legacy_completed'
      WHEN status = 'unavailable' THEN 'runner_unavailable'
      ELSE ''
    END,
    heartbeat_at = updated_at,
    started_at = created_at,
    completed_at = CASE
      WHEN status IN ('succeeded', 'failed', 'unavailable', 'cancelled') THEN updated_at
      ELSE NULL
    END,
    status = CASE
      WHEN status = 'running' THEN CASE WHEN purpose = 'deep_research' THEN 'queued' ELSE 'searching' END
      WHEN status = 'succeeded' THEN CASE WHEN purpose = 'deep_research' THEN 'partially_completed' ELSE 'completed' END
      WHEN status = 'unavailable' THEN 'failed'
      ELSE status
    END;

ALTER TABLE platform_research_runs
  ADD KEY idx_research_run_execution
    (organization_id, project_id, run_mode, status, updated_at),
  ADD CONSTRAINT chk_research_run_mode_v2
    CHECK (run_mode IN ('quick', 'deep')),
  ADD CONSTRAINT chk_research_run_status_v2
    CHECK (status IN (
      'queued', 'planning', 'searching', 'reading', 'cross_checking',
      'drafting', 'auditing', 'completed', 'partially_completed',
      'failed', 'cancelled'
    )),
  ADD CONSTRAINT chk_research_run_rounds_v2
    CHECK (current_round >= 0 AND max_rounds >= 1 AND max_rounds <= 20 AND current_round <= max_rounds),
  ADD CONSTRAINT chk_research_run_budgets_v2
    CHECK (time_budget_seconds >= 1 AND time_budget_seconds <= 86400 AND token_budget >= 1);

CREATE TABLE platform_research_iterations (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  research_run_id VARCHAR(96) NOT NULL,
  round_number INT NOT NULL,
  status VARCHAR(24) NOT NULL,
  objective VARCHAR(2000) NOT NULL,
  query_text VARCHAR(2000) NOT NULL,
  action_summary VARCHAR(2000) NOT NULL,
  source_ids JSON NOT NULL,
  artifact_ids JSON NOT NULL,
  finding_ids JSON NOT NULL,
  coverage_json JSON NOT NULL,
  open_gaps_json JSON NOT NULL,
  usage_json JSON NULL,
  input_hash VARCHAR(80) NOT NULL,
  output_hash VARCHAR(80) NOT NULL,
  error_code VARCHAR(128) NULL,
  error_message VARCHAR(1024) NULL,
  started_at DATETIME(6) NOT NULL,
  completed_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_research_iteration_round
    (organization_id, project_id, research_run_id, round_number),
  KEY idx_research_iteration_run
    (organization_id, project_id, research_run_id, round_number),
  CONSTRAINT chk_research_iteration_status
    CHECK (status IN ('running', 'completed', 'failed', 'cancelled')),
  CONSTRAINT chk_research_iteration_round
    CHECK (round_number >= 1 AND round_number <= 20)
);

CREATE TABLE platform_research_findings (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  contract_version VARCHAR(64) NOT NULL,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  research_run_id VARCHAR(96) NOT NULL,
  round_number INT NOT NULL,
  claim VARCHAR(2000) NOT NULL,
  status VARCHAR(24) NOT NULL,
  time_scope VARCHAR(96) NOT NULL,
  confidence VARCHAR(16) NOT NULL,
  supporting_source_ids JSON NOT NULL,
  conflicting_source_ids JSON NOT NULL,
  target_artifact VARCHAR(16) NOT NULL,
  target_field_path VARCHAR(160) NOT NULL,
  implication VARCHAR(2000) NOT NULL,
  proposed_value JSON NULL,
  content_hash VARCHAR(80) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_research_finding_hash
    (organization_id, project_id, research_run_id, content_hash),
  KEY idx_research_finding_run
    (organization_id, project_id, research_run_id, status, round_number),
  CONSTRAINT chk_research_finding_status
    CHECK (status IN ('tentative', 'verified', 'conflicting', 'invalid')),
  CONSTRAINT chk_research_finding_confidence
    CHECK (confidence IN ('low', 'medium', 'high')),
  CONSTRAINT chk_research_finding_target
    CHECK (target_artifact IN ('brief', 'strategy')),
  CONSTRAINT chk_research_finding_round
    CHECK (round_number >= 1 AND round_number <= 20)
);
