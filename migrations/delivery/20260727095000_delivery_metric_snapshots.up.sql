CREATE TABLE delivery_metric_snapshots (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  execution_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  plan_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  creative_package_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  is_simulated BOOLEAN NOT NULL,
  dataset_version VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  currency CHAR(3) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  window_start DATETIME(6) NOT NULL,
  window_end DATETIME(6) NOT NULL,
  impressions BIGINT NOT NULL,
  clicks BIGINT NOT NULL,
  conversions BIGINT NOT NULL,
  spend_cents BIGINT NOT NULL,
  created_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_delivery_metric_dataset (organization_id, execution_id, dataset_version),
  UNIQUE KEY uq_delivery_metric_scope (organization_id, id),
  KEY idx_delivery_metric_project (organization_id, project_id, created_at),
  CONSTRAINT fk_delivery_metric_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT fk_delivery_metric_execution FOREIGN KEY (organization_id, execution_id) REFERENCES delivery_executions(organization_id, id),
  CONSTRAINT fk_delivery_metric_plan FOREIGN KEY (organization_id, plan_id) REFERENCES delivery_plans(organization_id, id),
  CONSTRAINT chk_delivery_metric_source CHECK (source IN ('demo_fixture')),
  CONSTRAINT chk_delivery_metric_simulated CHECK (is_simulated = TRUE),
  CONSTRAINT chk_delivery_metric_currency CHECK (currency = 'CNY'),
  CONSTRAINT chk_delivery_metric_window CHECK (window_end > window_start),
  CONSTRAINT chk_delivery_metric_values CHECK (
    impressions >= 0 AND clicks >= 0 AND conversions >= 0 AND spend_cents >= 0
    AND clicks <= impressions AND conversions <= clicks
  )
);
