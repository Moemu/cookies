ALTER TABLE strategy_tasks
  ADD COLUMN discarded_at DATETIME(6) NULL AFTER status,
  ADD COLUMN discarded_by VARCHAR(96) NULL AFTER discarded_at,
  ADD COLUMN discard_reason VARCHAR(500) NULL AFTER discarded_by,
  ADD KEY idx_strategy_task_lifecycle
    (organization_id, project_id, discarded_at, updated_at);

ALTER TABLE strategy_drafts
  ADD COLUMN archived_at DATETIME(6) NULL AFTER status,
  ADD COLUMN archived_by VARCHAR(96) NULL AFTER archived_at,
  ADD COLUMN archive_reason VARCHAR(500) NULL AFTER archived_by,
  ADD KEY idx_strategy_draft_lifecycle
    (organization_id, project_id, archived_at, updated_at);
