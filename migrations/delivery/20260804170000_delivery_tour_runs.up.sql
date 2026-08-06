CREATE TABLE delivery_tour_runs (
  id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  owner_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  scenario VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  prepared_at DATETIME(6) NULL,
  reset_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (organization_id, project_id, id),
  KEY idx_delivery_tour_runs_owner (organization_id, project_id, owner_id, updated_at),
  CONSTRAINT fk_delivery_tour_runs_project
    FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT chk_delivery_tour_runs_status CHECK (status IN ('preparing', 'prepared', 'reset')),
  CONSTRAINT chk_delivery_tour_runs_source CHECK (source = 'mock'),
  CONSTRAINT chk_delivery_tour_runs_scenario CHECK (scenario = 'delivery_tour')
);

ALTER TABLE delivery_plans
  ADD COLUMN tour_run_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER scenario,
  ADD COLUMN tour_owner_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER tour_run_id,
  ADD COLUMN tour_case VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER tour_owner_id,
  ADD UNIQUE KEY uq_delivery_tour_plan_case (organization_id, project_id, tour_run_id, tour_case),
  ADD KEY idx_delivery_tour_plan_owner (organization_id, project_id, tour_run_id, tour_owner_id),
  ADD CONSTRAINT fk_delivery_plan_tour_run
    FOREIGN KEY (organization_id, project_id, tour_run_id) REFERENCES delivery_tour_runs(organization_id, project_id, id),
  ADD CONSTRAINT chk_delivery_tour_plan_scope CHECK (
    (tour_run_id IS NULL AND tour_owner_id IS NULL AND tour_case IS NULL)
    OR (tour_run_id IS NOT NULL AND tour_owner_id IS NOT NULL AND tour_case IS NOT NULL)
  );
