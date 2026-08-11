ALTER TABLE creative_tasks
  DROP INDEX uq_creative_tasks_lineage,
  DROP COLUMN lineage_key;
