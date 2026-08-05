DROP TABLE IF EXISTS strategy_creative_task_overlays;

DROP INDEX idx_strategy_task_plan_package_route ON strategy_creative_task_plans;

ALTER TABLE strategy_creative_task_plans
    DROP COLUMN selected_route_id,
    DROP COLUMN handoff_content_hash,
    DROP COLUMN handoff_contract_version,
    DROP COLUMN package_content_hash,
    DROP COLUMN package_version,
    DROP COLUMN package_id,
    DROP COLUMN contract_version;
