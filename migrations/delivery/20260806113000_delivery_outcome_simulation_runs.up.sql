CREATE TABLE delivery_simulation_runs (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  execution_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  plan_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  plan_version INT NOT NULL,
  plan_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  model_version VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  scenario VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  stable_seed VARCHAR(128) NOT NULL,
  input_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  fingerprint CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  input_json JSON NOT NULL,
  parameters_json JSON NOT NULL,
  events_json JSON NOT NULL,
  evidence_json JSON NOT NULL,
  status VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  completed_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_delivery_simulation_fingerprint (organization_id, project_id, fingerprint),
  UNIQUE KEY uq_delivery_simulation_scope (organization_id, id),
  KEY idx_delivery_simulation_execution (organization_id, project_id, execution_id, created_at),
  CONSTRAINT fk_delivery_simulation_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT fk_delivery_simulation_execution FOREIGN KEY (organization_id, execution_id) REFERENCES delivery_executions(organization_id, id),
  CONSTRAINT fk_delivery_simulation_plan FOREIGN KEY (organization_id, plan_id) REFERENCES delivery_plans(organization_id, id),
  CONSTRAINT chk_delivery_simulation_scenario CHECK (scenario IN ('steady','cost_pressure','under_delivery','creative_fatigue','tracking_anomaly','review_rejected')),
  CONSTRAINT chk_delivery_simulation_status CHECK (status = 'completed')
);

ALTER TABLE delivery_metric_snapshots
  ADD COLUMN simulation_run_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER execution_id,
  ADD COLUMN revenue_cents BIGINT NOT NULL DEFAULT 0 AFTER spend_cents,
  ADD COLUMN calculation_basis JSON NULL AFTER revenue_cents,
  ADD KEY idx_delivery_metric_simulation_run (organization_id, project_id, simulation_run_id),
  ADD CONSTRAINT fk_delivery_metric_simulation_run FOREIGN KEY (organization_id, simulation_run_id) REFERENCES delivery_simulation_runs(organization_id, id);

ALTER TABLE delivery_alerts
  ADD COLUMN simulation_run_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER execution_id,
  ADD KEY idx_delivery_alert_simulation_run (organization_id, project_id, simulation_run_id),
  ADD CONSTRAINT fk_delivery_alert_simulation_run FOREIGN KEY (organization_id, simulation_run_id) REFERENCES delivery_simulation_runs(organization_id, id);

ALTER TABLE delivery_alerts
  DROP CHECK chk_delivery_alert_type,
  ADD CONSTRAINT chk_delivery_alert_type CHECK (alert_type IN ('review_rejected','spend_spike','zero_conversion','cost_worsening','under_delivery','creative_fatigue','tracking_anomaly'));

ALTER TABLE delivery_recommendations
  ADD COLUMN simulation_run_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER plan_version,
  ADD KEY idx_delivery_recommendation_simulation_run (organization_id, project_id, simulation_run_id),
  ADD CONSTRAINT fk_delivery_recommendation_simulation_run FOREIGN KEY (organization_id, simulation_run_id) REFERENCES delivery_simulation_runs(organization_id, id);
