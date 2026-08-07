ALTER TABLE creative_tasks
  ADD COLUMN lineage_key CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER performance_mode,
  ADD UNIQUE KEY uq_creative_tasks_lineage (organization_id, project_id, lineage_key);
