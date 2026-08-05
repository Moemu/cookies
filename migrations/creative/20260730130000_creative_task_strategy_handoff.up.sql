ALTER TABLE creative_intakes
  DROP CHECK chk_creative_intakes_source;

ALTER TABLE creative_intakes
  ADD CONSTRAINT chk_creative_intakes_source
    CHECK (source_type IN ('manual', 'strategy_package', 'task_strategy', 'uploaded_document', 'conversation')),
  ADD COLUMN task_strategy_plan_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER strategy_package_content_hash,
  ADD COLUMN task_strategy_version BIGINT NULL AFTER task_strategy_plan_id,
  ADD COLUMN task_strategy_content_hash CHAR(71) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER task_strategy_version,
  ADD UNIQUE KEY uq_creative_intakes_task_strategy (
    organization_id,
    project_id,
    source_type,
    task_strategy_plan_id,
    task_strategy_version,
    task_strategy_content_hash
  );
