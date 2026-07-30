ALTER TABLE creative_tasks
  ADD CONSTRAINT uq_creative_tasks_single_intake UNIQUE (organization_id, intake_id);
