-- A Creative task is archived, never hard-deleted: draft history, frozen
-- CreativeVersions, Provider jobs and Asset lineage remain available for audit.
ALTER TABLE creative_tasks
  DROP CHECK chk_creative_tasks_status,
  ADD CONSTRAINT chk_creative_tasks_status CHECK (status IN ('draft', 'in_progress', 'ready_for_review', 'archived'));
