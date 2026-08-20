-- Immutable Ocean Engine point-in-time ledger. Application roles receive
-- INSERT and SELECT only. UPDATE and DELETE are intentionally unsupported.
CREATE TABLE connector_sync_runs (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  account_ref VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  schema_version VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  cursor_ref VARCHAR(512) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  attempt INT UNSIGNED NOT NULL DEFAULT 1,
  started_at DATETIME(6) NOT NULL,
  completed_at DATETIME(6) NULL,
  CONSTRAINT chk_connector_sync_status CHECK (status IN ('queued', 'running', 'completed', 'failed'))
);

CREATE TABLE connector_raw_snapshots (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_system VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_ref VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  ingest_run_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  schema_version VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  endpoint_key VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  request_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  payload_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  encrypted_evidence LONGBLOB NOT NULL,
  key_version VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  collected_at DATETIME(6) NOT NULL,
  available_at DATETIME(6) NOT NULL,
  data_through DATETIME(6) NOT NULL,
  valid_from DATETIME(6) NOT NULL,
  valid_to DATETIME(6) NULL,
  quality_status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  UNIQUE KEY uq_connector_raw_idempotency (source_system, source_ref, ingest_run_id, request_hash, payload_hash),
  CONSTRAINT fk_connector_raw_run FOREIGN KEY (ingest_run_id) REFERENCES connector_sync_runs(id),
  CONSTRAINT chk_connector_raw_quality CHECK (quality_status IN ('accept', 'reject', 'quarantine', 'warning')),
  CONSTRAINT chk_connector_raw_validity CHECK (valid_to IS NULL OR valid_to > valid_from)
);

CREATE TABLE connector_object_snapshots (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_system VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_ref VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  object_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  object_ref VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  parent_ref VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  ingest_run_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  raw_snapshot_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  schema_version VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  payload_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  state_json JSON NOT NULL,
  collected_at DATETIME(6) NOT NULL, available_at DATETIME(6) NOT NULL, data_through DATETIME(6) NOT NULL,
  valid_from DATETIME(6) NOT NULL, valid_to DATETIME(6) NULL,
  quality_status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  UNIQUE KEY uq_connector_object_idempotency (source_system, object_kind, object_ref, valid_from, payload_hash),
  KEY ix_connector_object_pit (organization_id, project_id, object_ref, available_at),
  CONSTRAINT fk_connector_object_raw FOREIGN KEY (raw_snapshot_id) REFERENCES connector_raw_snapshots(id)
);

CREATE TABLE connector_configuration_snapshots LIKE connector_object_snapshots;
ALTER TABLE connector_configuration_snapshots
  DROP INDEX uq_connector_object_idempotency,
  DROP INDEX ix_connector_object_pit,
  ADD UNIQUE KEY uq_connector_config_idempotency (source_system, object_ref, valid_from, payload_hash),
  ADD KEY ix_connector_config_pit (organization_id, project_id, object_ref, available_at);

CREATE TABLE connector_configuration_change_events (
  id CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_system VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_ref VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  object_ref VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  ingest_run_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  schema_version VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  payload_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  field_path VARCHAR(512) NOT NULL,
  old_value_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  new_value_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  before_snapshot_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  after_snapshot_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  observed_at DATETIME(6) NOT NULL,
  collected_at DATETIME(6) NOT NULL, available_at DATETIME(6) NOT NULL, data_through DATETIME(6) NOT NULL,
  valid_from DATETIME(6) NOT NULL, valid_to DATETIME(6) NULL,
  quality_status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  UNIQUE KEY uq_connector_change_deterministic (before_snapshot_id, after_snapshot_id, field_path, payload_hash)
);

CREATE TABLE connector_metric_windows (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_system VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_ref VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  object_ref VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  ingest_run_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  raw_snapshot_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  schema_version VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  payload_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  window_start DATETIME(6) NOT NULL, window_end DATETIME(6) NOT NULL,
  granularity VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  platform_timezone VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  attribution_window VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  metric_definition_version VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  currency CHAR(3) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  amount_unit VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  metrics_json JSON NOT NULL,
  revision_of VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  collected_at DATETIME(6) NOT NULL, available_at DATETIME(6) NOT NULL, data_through DATETIME(6) NOT NULL,
  valid_from DATETIME(6) NOT NULL, valid_to DATETIME(6) NULL,
  quality_status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  UNIQUE KEY uq_connector_metric_revision (source_system, object_ref, window_start, window_end, attribution_window, metric_definition_version, payload_hash),
  KEY ix_connector_metric_pit (organization_id, project_id, object_ref, window_start, window_end, available_at),
  CONSTRAINT fk_connector_metric_revision FOREIGN KEY (revision_of) REFERENCES connector_metric_windows(id),
  CONSTRAINT chk_connector_metric_window CHECK (window_end > window_start)
);

CREATE TABLE connector_material_bindings LIKE connector_object_snapshots;
ALTER TABLE connector_material_bindings ADD COLUMN material_ref VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, ADD COLUMN promotion_ref VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL;

CREATE TABLE connector_material_metric_windows LIKE connector_metric_windows;

CREATE TABLE connector_platform_status_events LIKE connector_object_snapshots;
ALTER TABLE connector_platform_status_events ADD COLUMN platform_status VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, ADD COLUMN status_reason VARCHAR(512) NOT NULL DEFAULT '';

CREATE TABLE connector_platform_diagnosis_snapshots LIKE connector_object_snapshots;
ALTER TABLE connector_platform_diagnosis_snapshots ADD COLUMN eligible_as_prelaunch_feature BOOLEAN NOT NULL DEFAULT FALSE, ADD CONSTRAINT chk_connector_diagnosis_prelaunch CHECK (eligible_as_prelaunch_feature = FALSE);

CREATE TABLE connector_conversion_revisions LIKE connector_metric_windows;
