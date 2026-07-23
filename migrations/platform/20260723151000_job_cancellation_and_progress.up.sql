ALTER TABLE platform_jobs
  ADD COLUMN cancel_requested_at DATETIME(6) NULL AFTER cancellable,
  ADD COLUMN progress_message VARCHAR(512) NULL AFTER progress;
