CREATE TABLE delivery_mechanistic_simulation_runs (
    id VARCHAR(80) PRIMARY KEY,
    organization_id VARCHAR(64) NOT NULL,
    project_id VARCHAR(64) NOT NULL,
    plan_id VARCHAR(64) NOT NULL,
    plan_version INT NOT NULL,
    fingerprint CHAR(64) NOT NULL,
    input_snapshot_hash CHAR(64) NOT NULL,
    model_version VARCHAR(80) NOT NULL,
    prior_set_version VARCHAR(80) NOT NULL,
    stable_seed VARCHAR(128) NOT NULL,
    result_json JSON NOT NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    UNIQUE KEY uq_delivery_mechanistic_simulation_fingerprint (organization_id, project_id, fingerprint),
    KEY idx_delivery_mechanistic_simulation_plan (organization_id, project_id, plan_id, plan_version),
    CONSTRAINT chk_delivery_mechanistic_simulation_plan_version CHECK (plan_version > 0)
);
