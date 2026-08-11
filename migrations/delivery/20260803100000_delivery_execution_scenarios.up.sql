-- A04 preserves existing A03 execution rows while adding durable execution
-- request identity, recovery metadata, and independently queryable steps.
ALTER TABLE delivery_executions
  MODIFY COLUMN completed_at DATETIME(6) NULL,
  ADD COLUMN approval_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER change_set_id,
  ADD COLUMN version BIGINT NOT NULL DEFAULT 1 AFTER status,
  ADD COLUMN adapter VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'mock_ocean_engine' AFTER execution_mode,
  ADD COLUMN source VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'mock' AFTER adapter,
  ADD COLUMN scenario VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'success' AFTER source,
  ADD COLUMN idempotency_key VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER scenario,
  ADD COLUMN request_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER idempotency_key,
  ADD COLUMN retry_allowed BOOLEAN NOT NULL DEFAULT FALSE AFTER completed_at,
  ADD COLUMN recovery_action VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER retry_allowed,
  ADD COLUMN recovery_reason VARCHAR(1000) NULL AFTER recovery_action,
  ADD COLUMN compensation_candidates JSON NULL AFTER recovery_reason,
  ADD UNIQUE KEY uq_delivery_execution_idempotency (organization_id, project_id, idempotency_key),
  ADD UNIQUE KEY uq_delivery_executions_project_scope (organization_id, project_id, id),
  ADD KEY idx_delivery_executions_project_started (organization_id, project_id, started_at),
  DROP CHECK chk_delivery_execution_status,
  ADD CONSTRAINT chk_delivery_execution_status CHECK (status IN ('queued', 'validating_approval', 'executing', 'verifying', 'succeeded', 'failed', 'partial', 'result_unknown', 'cancelled')),
  ADD CONSTRAINT chk_delivery_execution_scenario CHECK (scenario IN ('success', 'failed', 'partial', 'result_unknown')),
  ADD CONSTRAINT chk_delivery_execution_source CHECK (source = 'mock'),
  ADD CONSTRAINT chk_delivery_execution_adapter CHECK (adapter = 'mock_ocean_engine');

ALTER TABLE delivery_evidence
  ADD COLUMN source VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'mock' AFTER evidence_mode,
  ADD COLUMN scenario VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'success' AFTER source,
  ADD COLUMN references_json JSON NULL AFTER scenario;

CREATE TABLE delivery_execution_steps (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  execution_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  sequence_number INT NOT NULL,
  action VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  attempt INT NOT NULL,
  effect VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  outcome_summary VARCHAR(1000) NOT NULL,
  evidence_ref VARCHAR(512) NULL,
  started_at DATETIME(6) NULL,
  completed_at DATETIME(6) NULL,
  version BIGINT NOT NULL,
  UNIQUE KEY uq_delivery_execution_step_sequence (organization_id, execution_id, sequence_number),
  KEY idx_delivery_execution_steps_project (organization_id, project_id, execution_id),
  CONSTRAINT fk_delivery_execution_steps_execution FOREIGN KEY (organization_id, project_id, execution_id) REFERENCES delivery_executions(organization_id, project_id, id),
  CONSTRAINT chk_delivery_execution_step_sequence CHECK (sequence_number > 0),
  CONSTRAINT chk_delivery_execution_step_attempt CHECK (attempt >= 0),
  CONSTRAINT chk_delivery_execution_step_version CHECK (version > 0),
  CONSTRAINT chk_delivery_execution_step_effect CHECK (effect IN ('confirmed_applied', 'confirmed_not_applied', 'unknown', 'none')),
  CONSTRAINT chk_delivery_execution_step_status CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'result_unknown', 'skipped'))
);
