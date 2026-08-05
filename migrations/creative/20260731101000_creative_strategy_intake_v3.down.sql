DROP INDEX uq_creative_intakes_strategy_input ON creative_intakes;

CREATE UNIQUE INDEX uq_creative_intakes_strategy_package
    ON creative_intakes (
        organization_id, project_id, source_type,
        strategy_package_id, strategy_package_version,
        strategy_package_content_hash
    );

ALTER TABLE creative_intakes
    DROP COLUMN input_identity_hash,
    DROP COLUMN task_overlay_content_hash,
    DROP COLUMN task_overlay_id,
    DROP COLUMN task_overlay_identity,
    DROP COLUMN handoff_content_hash,
    DROP COLUMN handoff_contract_version,
    DROP COLUMN selected_route_id,
    DROP COLUMN contract_version;
