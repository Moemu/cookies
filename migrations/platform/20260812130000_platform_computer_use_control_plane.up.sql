CREATE TABLE computer_use_environments (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  platform VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  account_id VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  mode VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  browser_version VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  region VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  healthy BOOLEAN NOT NULL,
  version BIGINT NOT NULL,
  UNIQUE KEY uq_computer_use_environment_scope (organization_id, project_id, id),
  KEY idx_computer_use_environment_account (organization_id, project_id, platform, account_id),
  CONSTRAINT fk_computer_use_environment_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT chk_computer_use_environment_platform CHECK (platform IN ('ocean_engine')),
  CONSTRAINT chk_computer_use_environment_mode CHECK (mode IN ('local_visible')),
  CONSTRAINT chk_computer_use_environment_version CHECK (version > 0)
);

CREATE TABLE computer_use_browser_profiles (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  environment_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  platform VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  account_id VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  state VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  version BIGINT NOT NULL,
  UNIQUE KEY uq_computer_use_profile_scope (organization_id, project_id, id),
  KEY idx_computer_use_profile_account (organization_id, project_id, platform, account_id),
  CONSTRAINT fk_computer_use_profile_environment FOREIGN KEY (organization_id, project_id, environment_id) REFERENCES computer_use_environments(organization_id, project_id, id),
  CONSTRAINT chk_computer_use_profile_platform CHECK (platform IN ('ocean_engine')),
  CONSTRAINT chk_computer_use_profile_state CHECK (state IN ('ready', 'takeover_required', 'disabled')),
  CONSTRAINT chk_computer_use_profile_version CHECK (version > 0)
);

CREATE TABLE computer_use_site_policies (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  platform VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  account_id VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  allowed_protocols JSON NOT NULL,
  allowed_hosts JSON NOT NULL,
  allowed_page_kinds JSON NOT NULL,
  allowed_platform_project_ids JSON NOT NULL,
  version BIGINT NOT NULL,
  UNIQUE KEY uq_computer_use_policy_scope (organization_id, project_id, id),
  KEY idx_computer_use_policy_account (organization_id, project_id, platform, account_id),
  CONSTRAINT fk_computer_use_policy_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT chk_computer_use_policy_platform CHECK (platform IN ('ocean_engine')),
  CONSTRAINT chk_computer_use_policy_version CHECK (version > 0)
);

CREATE TABLE computer_use_kill_switches (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  scope VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  scope_key VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  platform VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NULL,
  active BOOLEAN NOT NULL,
  reason VARCHAR(1000) NOT NULL,
  version BIGINT NOT NULL,
  updated_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_computer_use_kill_switch_scope (scope, scope_key),
  CONSTRAINT chk_computer_use_kill_switch_scope CHECK (scope IN ('global', 'platform', 'organization')),
  CONSTRAINT chk_computer_use_kill_switch_platform CHECK (platform IS NULL OR platform IN ('ocean_engine')),
  CONSTRAINT chk_computer_use_kill_switch_version CHECK (version > 0)
);

CREATE TABLE computer_use_runs (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  platform VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  account_id VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  authority_json JSON NOT NULL,
  environment_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  profile_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  lease_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  policy_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  state VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  blocking_reason VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  paused BOOLEAN NOT NULL,
  takeover_active BOOLEAN NOT NULL,
  version BIGINT NOT NULL,
  idempotency_key VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  request_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_computer_use_run_scope (organization_id, project_id, id),
  UNIQUE KEY uq_computer_use_run_idempotency (organization_id, project_id, idempotency_key),
  KEY idx_computer_use_run_account (organization_id, project_id, platform, account_id, updated_at),
  CONSTRAINT fk_computer_use_run_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT fk_computer_use_run_environment FOREIGN KEY (organization_id, project_id, environment_id) REFERENCES computer_use_environments(organization_id, project_id, id),
  CONSTRAINT fk_computer_use_run_profile FOREIGN KEY (organization_id, project_id, profile_id) REFERENCES computer_use_browser_profiles(organization_id, project_id, id),
  CONSTRAINT fk_computer_use_run_policy FOREIGN KEY (organization_id, project_id, policy_id) REFERENCES computer_use_site_policies(organization_id, project_id, id),
  CONSTRAINT chk_computer_use_run_platform CHECK (platform IN ('ocean_engine')),
  CONSTRAINT chk_computer_use_run_state CHECK (state IN ('queued','environment_check','awaiting_takeover','preparing','awaiting_confirmation','submitting','verifying','succeeded','failed','partial','result_unknown','cancelled')),
  CONSTRAINT chk_computer_use_run_version CHECK (version > 0)
);

CREATE TABLE computer_use_session_leases (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  run_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  environment_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  profile_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  platform VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  account_id VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  holder VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  active_lock_key VARCHAR(512) CHARACTER SET ascii COLLATE ascii_bin NULL,
  fencing_token BIGINT NOT NULL,
  version BIGINT NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  heartbeat_deadline DATETIME(6) NOT NULL,
  released_at DATETIME(6) NULL,
  UNIQUE KEY uq_computer_use_lease_scope (organization_id, project_id, id),
  UNIQUE KEY uq_computer_use_lease_active (active_lock_key),
  KEY idx_computer_use_lease_run (organization_id, project_id, run_id),
  CONSTRAINT fk_computer_use_lease_run FOREIGN KEY (organization_id, project_id, run_id) REFERENCES computer_use_runs(organization_id, project_id, id),
  CONSTRAINT chk_computer_use_lease_platform CHECK (platform IN ('ocean_engine')),
  CONSTRAINT chk_computer_use_lease_fence CHECK (fencing_token > 0 AND version > 0),
  CONSTRAINT chk_computer_use_lease_times CHECK (expires_at > heartbeat_deadline)
);

ALTER TABLE computer_use_runs
  ADD CONSTRAINT fk_computer_use_run_lease FOREIGN KEY (organization_id, project_id, lease_id) REFERENCES computer_use_session_leases(organization_id, project_id, id);

CREATE TABLE computer_use_run_steps (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  run_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  sequence_number INT NOT NULL,
  workflow_step_id VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  action VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  blocking_reason VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  attempt INT NOT NULL,
  version BIGINT NOT NULL,
  UNIQUE KEY uq_computer_use_step_scope (organization_id, project_id, id),
  UNIQUE KEY uq_computer_use_step_sequence (organization_id, project_id, run_id, sequence_number),
  CONSTRAINT fk_computer_use_step_run FOREIGN KEY (organization_id, project_id, run_id) REFERENCES computer_use_runs(organization_id, project_id, id),
  CONSTRAINT chk_computer_use_step_status CHECK (status IN ('pending','running','succeeded','failed','result_unknown','skipped')),
  CONSTRAINT chk_computer_use_step_numbers CHECK (sequence_number > 0 AND attempt > 0 AND version > 0)
);

CREATE TABLE computer_use_events (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  run_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  sequence_number BIGINT NOT NULL,
  kind VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  summary VARCHAR(1000) NOT NULL,
  actor VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_computer_use_event_sequence (organization_id, project_id, run_id, sequence_number),
  CONSTRAINT fk_computer_use_event_run FOREIGN KEY (organization_id, project_id, run_id) REFERENCES computer_use_runs(organization_id, project_id, id)
);

CREATE TABLE computer_use_evidence (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  run_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  step_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  evidence_json JSON NOT NULL,
  object_fingerprint CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  skill_version VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  selector_version VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  action_version VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  redaction_version VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_computer_use_evidence_scope (organization_id, project_id, id),
  KEY idx_computer_use_evidence_run (organization_id, project_id, run_id, created_at),
  CONSTRAINT fk_computer_use_evidence_step FOREIGN KEY (organization_id, project_id, step_id) REFERENCES computer_use_run_steps(organization_id, project_id, id)
);

CREATE TABLE computer_use_final_confirmations (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  run_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  binding_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  token_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  issued_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  issued_at DATETIME(6) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  consumed_at DATETIME(6) NULL,
  rejected_at DATETIME(6) NULL,
  invalidated_at DATETIME(6) NULL,
  version BIGINT NOT NULL,
  UNIQUE KEY uq_computer_use_confirmation_scope (organization_id, project_id, id),
  UNIQUE KEY uq_computer_use_confirmation_token (token_digest),
  KEY idx_computer_use_confirmation_run (organization_id, project_id, run_id, issued_at),
  CONSTRAINT fk_computer_use_confirmation_run FOREIGN KEY (organization_id, project_id, run_id) REFERENCES computer_use_runs(organization_id, project_id, id),
  CONSTRAINT chk_computer_use_confirmation_version CHECK (version > 0),
  CONSTRAINT chk_computer_use_confirmation_expiry CHECK (expires_at > issued_at)
);

CREATE TABLE computer_use_confirmation_events (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  run_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  confirmation_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  actor VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  reason VARCHAR(1000) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  KEY idx_computer_use_confirmation_event (organization_id, project_id, confirmation_id, created_at),
  CONSTRAINT fk_computer_use_confirmation_event_confirmation FOREIGN KEY (organization_id, project_id, confirmation_id) REFERENCES computer_use_final_confirmations(organization_id, project_id, id),
  CONSTRAINT chk_computer_use_confirmation_event_kind CHECK (kind IN ('issued','consumed','rejected','invalidated','expired'))
);

CREATE TABLE computer_use_controlled_action_attempts (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  run_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  step_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  confirmation_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  approval_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  lease_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  fencing_token BIGINT NOT NULL,
  action_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  idempotency_key VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_computer_use_attempt_scope (organization_id, project_id, id),
  UNIQUE KEY uq_computer_use_attempt_confirmation (organization_id, project_id, run_id, confirmation_id),
  UNIQUE KEY uq_computer_use_attempt_idempotency (organization_id, project_id, idempotency_key),
  CONSTRAINT fk_computer_use_attempt_step FOREIGN KEY (organization_id, project_id, step_id) REFERENCES computer_use_run_steps(organization_id, project_id, id),
  CONSTRAINT fk_computer_use_attempt_confirmation FOREIGN KEY (organization_id, project_id, confirmation_id) REFERENCES computer_use_final_confirmations(organization_id, project_id, id),
  CONSTRAINT fk_computer_use_attempt_lease FOREIGN KEY (organization_id, project_id, lease_id) REFERENCES computer_use_session_leases(organization_id, project_id, id),
  CONSTRAINT chk_computer_use_attempt_status CHECK (status IN ('authorized','submitted','result_unknown','verified','failed')),
  CONSTRAINT chk_computer_use_attempt_fence CHECK (fencing_token > 0)
);
