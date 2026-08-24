-- Safe account/product launch-batch priors. Raw account IDs and report rows stay outside this table.
CREATE TABLE connector_launch_batch_calibrations (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  account_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  schema_version VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  model_version VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  payload_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  prior_json JSON NOT NULL,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_connector_launch_batch_calibration_payload (organization_id, account_id, payload_hash),
  KEY idx_connector_launch_batch_calibration_latest (organization_id, account_id, created_at, id),
  CONSTRAINT fk_connector_launch_batch_calibration_account FOREIGN KEY (organization_id, account_id) REFERENCES platform_accounts(organization_id, id),
  CONSTRAINT chk_connector_launch_batch_calibration_status CHECK (status IN ('ready_for_probabilistic_shadow'))
);
