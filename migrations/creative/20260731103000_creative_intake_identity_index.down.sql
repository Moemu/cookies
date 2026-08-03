DROP INDEX uq_creative_intakes_strategy_input ON creative_intakes;

CREATE UNIQUE INDEX uq_creative_intakes_strategy_input
    ON creative_intakes (
        organization_id, project_id, source_type,
        strategy_package_id, strategy_package_version,
        strategy_package_content_hash, handoff_content_hash,
        selected_route_id, task_overlay_identity
    );
