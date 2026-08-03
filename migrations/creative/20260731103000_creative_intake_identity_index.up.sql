DROP INDEX uq_creative_intakes_strategy_input ON creative_intakes;

CREATE UNIQUE INDEX uq_creative_intakes_strategy_input
    ON creative_intakes (
        organization_id, project_id, source_type, input_identity_hash
    );
