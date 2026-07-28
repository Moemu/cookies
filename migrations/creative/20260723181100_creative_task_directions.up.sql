ALTER TABLE creative_tasks
  -- The previous unique key also happened to be the only index MySQL could
  -- use for fk_creative_tasks_intake. Preserve that FK support before making
  -- the relationship one-to-many.
  ADD KEY idx_creative_tasks_intake_fk (organization_id, intake_id),
  DROP INDEX uq_creative_tasks_single_intake;
