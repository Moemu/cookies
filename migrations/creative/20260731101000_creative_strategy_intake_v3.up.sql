ALTER TABLE creative_intakes
    ADD COLUMN contract_version VARCHAR(64) NOT NULL DEFAULT 'creative-intake/v1',
    ADD COLUMN selected_route_id VARCHAR(96) NULL,
    ADD COLUMN handoff_contract_version VARCHAR(64) NULL,
    ADD COLUMN handoff_content_hash VARCHAR(71) NULL,
    ADD COLUMN task_overlay_id VARCHAR(96) NULL,
    ADD COLUMN task_overlay_content_hash VARCHAR(71) NULL,
    ADD COLUMN task_overlay_identity VARCHAR(96) NOT NULL DEFAULT '',
    ADD COLUMN input_identity_hash VARCHAR(71) NULL;

DROP INDEX uq_creative_intakes_strategy_package ON creative_intakes;

CREATE UNIQUE INDEX uq_creative_intakes_strategy_input
    ON creative_intakes (
        organization_id, project_id, source_type,
        strategy_package_id, strategy_package_version,
        strategy_package_content_hash, handoff_content_hash,
        selected_route_id, task_overlay_identity
    );
