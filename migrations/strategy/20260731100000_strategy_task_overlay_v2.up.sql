ALTER TABLE strategy_creative_task_plans
    ADD COLUMN contract_version VARCHAR(64) NOT NULL DEFAULT 'strategy-creative-task-plan/v1',
    ADD COLUMN package_id VARCHAR(96) NULL,
    ADD COLUMN package_version BIGINT NULL,
    ADD COLUMN package_content_hash VARCHAR(71) NULL,
    ADD COLUMN handoff_contract_version VARCHAR(64) NULL,
    ADD COLUMN handoff_content_hash VARCHAR(71) NULL,
    ADD COLUMN selected_route_id VARCHAR(96) NULL;

CREATE INDEX idx_strategy_task_plan_package_route
    ON strategy_creative_task_plans (
        organization_id, project_id, package_id, package_version, selected_route_id
    );

CREATE TABLE strategy_creative_task_overlays (
    organization_id VARCHAR(96) NOT NULL,
    project_id VARCHAR(96) NOT NULL,
    overlay_id VARCHAR(96) NOT NULL,
    plan_id VARCHAR(96) NOT NULL,
    task_strategy_version BIGINT NOT NULL,
    package_id VARCHAR(96) NOT NULL,
    package_version BIGINT NOT NULL,
    package_content_hash VARCHAR(71) NOT NULL,
    handoff_contract_version VARCHAR(64) NOT NULL,
    handoff_content_hash VARCHAR(71) NOT NULL,
    selected_route_id VARCHAR(96) NOT NULL,
    contract_version VARCHAR(64) NOT NULL,
    snapshot JSON NOT NULL,
    content_hash VARCHAR(71) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (organization_id, project_id, overlay_id),
    UNIQUE KEY uq_strategy_task_overlay_version (
        organization_id, project_id, plan_id, task_strategy_version
    ),
    CONSTRAINT fk_strategy_task_overlay_plan
        FOREIGN KEY (organization_id, project_id, plan_id)
        REFERENCES strategy_creative_task_plans (organization_id, project_id, id)
);
